from dataclasses import dataclass
from typing import List
import numpy as np
from sentence_transformers import SentenceTransformer, CrossEncoder
from datasets import load_dataset
from rank_bm25 import BM25Okapi
from openai import AsyncOpenAI
import os
from ragas.llms import llm_factory

@dataclass
class RetrievalResult:
    texts: list[str]
    scores: list[float]

class DenseRetriever:
    def __init__(self, model_name: str = "all-MiniLM-L6-v2"):
        self.encoder = SentenceTransformer(model_name)
        self.corpus_embeddings = None
        self.corpus = None
    
    def index(self, corpus: List[str]):
        self.corpus = corpus
        self.corpus_embeddings = self.encoder.encode(corpus, convert_to_numpy=True)
    
    def retrieve(self, query: str, top_k: int = 5) -> RetrievalResult:
        query_emb = self.encoder.encode([query], convert_to_numpy=True)
        scores = np.dot(self.corpus_embeddings, query_emb.T).flatten()
        top_indices = scores.argsort()[-top_k:][::-1]
        return RetrievalResult(
            texts=[self.corpus[i] for i in top_indices],
            scores=[float(scores[i]) for i in top_indices]
        )

class BM25Retriever:
    def __init__(self):
        self.bm25 = None
        self.corpus = None
    
    def index(self, corpus: List[str]):
        self.corpus = corpus
        tokenized = [doc.lower().split() for doc in corpus]
        self.bm25 = BM25Okapi(tokenized)
    
    def retrieve(self, query: str, top_k: int = 5) -> RetrievalResult:
        tokenized_query = query.lower().split()
        scores = self.bm25.get_scores(tokenized_query)
        top_indices = scores.argsort()[-top_k:][::-1]
        return RetrievalResult(
            texts=[self.corpus[i] for i in top_indices],
            scores=[float(scores[i]) for i in top_indices]
        )

class HybridRetriever:
    def __init__(self, dense_weight: float = 0.5, k: int = 60):
        self.dense = DenseRetriever()
        self.bm25 = BM25Retriever()
        self.dense_weight = dense_weight
        self.k = k  # RRF parameter
    
    def index(self, corpus: List[str]):
        self.dense.index(corpus)
        self.bm25.index(corpus)
        self.corpus = corpus
    
    def _reciprocal_rank_fusion(
        self, 
        dense_results: RetrievalResult,
        bm25_results: RetrievalResult,
        top_k: int
    ) -> RetrievalResult:
        """RRF: score = sum(1 / (k + rank))"""
        scores = {}
        
        # Score from dense retrieval
        for rank, text in enumerate(dense_results.texts):
            scores[text] = scores.get(text, 0) + self.dense_weight / (self.k + rank + 1)
        
        # Score from BM25 retrieval
        bm25_weight = 1 - self.dense_weight
        for rank, text in enumerate(bm25_results.texts):
            scores[text] = scores.get(text, 0) + bm25_weight / (self.k + rank + 1)
        
        # Sort by combined score
        sorted_docs = sorted(scores.items(), key=lambda x: x[1], reverse=True)[:top_k]
        return RetrievalResult(
            texts=[doc for doc, _ in sorted_docs],
            scores=[score for _, score in sorted_docs]
        )
    
    def retrieve(self, query: str, top_k: int = 5) -> RetrievalResult:
        # Retrieve more candidates for fusion
        dense_results = self.dense.retrieve(query, top_k=top_k * 3)
        bm25_results = self.bm25.retrieve(query, top_k=top_k * 3)
        return self._reciprocal_rank_fusion(dense_results, bm25_results, top_k)

class CrossEncoderReranker:
    """MS MARCO Cross-Encoder reranker"""
    
    def __init__(self, model_name: str = "cross-encoder/ms-marco-MiniLM-L-6-v2"):
        self.model = CrossEncoder(model_name)
    
    def rerank(self, query: str, documents: List[str], top_k: int = 5) -> RetrievalResult:
        pairs = [[query, doc] for doc in documents]
        scores = self.model.predict(pairs)
        
        sorted_indices = np.argsort(scores)[::-1][:top_k]
        return RetrievalResult(
            texts=[documents[i] for i in sorted_indices],
            scores=[float(scores[i]) for i in sorted_indices]
        )

client = AsyncOpenAI(api_key=os.environ["OPENAI_API_KEY"])

async def run_prompt(question: str, contexts: List[str]) -> str:
    context_str = "\n\n".join(contexts)
    prompt = f"""Answer the question based on the context provided. Keep the answer concise and to the point.

Context:
{context_str}

Question: {question}

Answer:"""

    response = await client.chat.completions.create(
        model="gpt-5-nano",
        messages=[
            {"role": "system", "content": "You are a helpful assistant that answers questions based on the context provided."},
            {"role": "user", "content": prompt},
        ],
    )
    response = (
        response.choices[0].message.content.strip()
        if response.choices[0].message.content
        else ""
    )
    return response

async def run_evals():
    from ragas import EvaluationDataset, SingleTurnSample, evaluate
    from ragas.metrics import (
        Faithfulness, 
        ContextPrecision,
        ContextRecall,
    )
    print("Loading datasets...")
    corpus_ds = load_dataset("rag-datasets/rag-mini-wikipedia", "text-corpus", split="passages", cache_dir="data")
    qa_ds = load_dataset("rag-datasets/rag-mini-wikipedia", "question-answer", split="test", cache_dir="data")

    corpus = [doc["passage"] for doc in corpus_ds]
    print("Indexing corpus...")
    hybrid_retriever = HybridRetriever()
    hybrid_retriever.index(corpus)
    cross_encoder_reranker = CrossEncoderReranker()

    questions = [item["question"] for item in qa_ds.select(range(10))]
    ground_truths = [item["answer"] for item in qa_ds.select(range(10))]

    results = []
    print("Running evaluations...")
    for question, ground_truth in zip(questions, ground_truths):
        retrieval_result = hybrid_retriever.retrieve(question)
        reranked_result = cross_encoder_reranker.rerank(question, retrieval_result.texts)
        response = await run_prompt(question, reranked_result.texts)
        print(question)
        print(reranked_result.texts)
        print(response)
        results.append(SingleTurnSample(
            user_input=question,
            retrieved_contexts=retrieval_result.texts,
            response=response,
            reference=ground_truth,
        ))
    print("Evaluating...")
    eval_dataset = EvaluationDataset(samples=results)
    print("Evaluating metrics...")
    wrapped_llm = llm_factory('gpt-4o-mini', client=client)
    results = evaluate(eval_dataset, metrics=[
        Faithfulness(llm=wrapped_llm),
        ContextPrecision(llm=wrapped_llm),
        ContextRecall(llm=wrapped_llm),
    ])
    print("Saving results...")
    df = results.to_pandas()
    df.to_csv("rag_evals_results.csv")
    return df

if __name__ == "__main__":
    import asyncio
    asyncio.run(run_evals())
