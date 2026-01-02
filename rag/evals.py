from datasets import load_dataset
import os
from openai import AsyncOpenAI
import datetime

client = AsyncOpenAI(api_key=os.environ["OPENAI_API_KEY"])

def load_prompt(prompt_file: str) -> str:
    """Load prompt from a text file"""
    with open(prompt_file, "r", encoding="utf-8") as f:
        return f.read().strip()


async def run_prompt(question_text: str, context: str, prompt_file: str = "evals_promptv1.txt"):
    """Run the prompt against a question with context"""
    system_prompt = load_prompt(prompt_file)
    user_message = f'Question: "{question_text}"\nContext: "{context}"'

    response = await client.chat.completions.create(
        model="gpt-5-nano-2025-08-07",
        response_format={"type": "json_object"},
        messages=[
            {"role": "system", "content": system_prompt},
            {"role": "user", "content": user_message},
        ],
    )
    response = (
        response.choices[0].message.content.strip()
        if response.choices[0].message.content
        else ""
    )
    return response

import json
from ragas.metrics.discrete import discrete_metric
from ragas.metrics.result import MetricResult

@discrete_metric(name="answerable_accuracy", allowed_values=["correct", "incorrect"])
def answerable_accuracy(prediction: str, expected_answerable: bool):
    try:
        answerable = json.loads(prediction).get("answerable")
        return MetricResult(
            value="correct" if answerable == expected_answerable else "incorrect",
            reason=f"Expected={expected_answerable}; Got={answerable}",
        )
    except Exception as e:
        return MetricResult(value="incorrect", reason=f"Parse error: {e}")

@discrete_metric(name="answer_accuracy", allowed_values=["correct", "incorrect"])
def answer_accuracy(prediction: str, expected_answer: str):
    try:
        answer = json.loads(prediction).get("answer")
        return MetricResult(
            value="correct" if answer == expected_answer else "incorrect",
            reason=f"Expected={expected_answer}; Got={answer}",
        )
    except Exception as e:
        return MetricResult(value="incorrect", reason=f"Parse error: {e}")

import asyncio, json
from ragas import experiment

@experiment()
async def squad_experiment(row, prompt_file: str, experiment_name: str):
    response = await run_prompt(
        row["question"],
        row["context"],
        prompt_file=prompt_file,
    )
    try:
        parsed = json.loads(response)
        predicted_answer = parsed.get("answer")
        predicted_answerable = parsed.get("answerable")
    except Exception:
        predicted_answer = None
        predicted_answerable = None

    expected_texts = row.get("answers", {}).get("text", []) or []
    expected_answerable = bool(expected_texts)
    expected_answer = expected_texts[0] if expected_texts else ""

    return {
        "id": row["id"],
        "question": row["question"],
        "context": row["context"],
        "response": response,
        "experiment_name": experiment_name,
        "expected_answer": expected_answer,
        "expected_answerable": expected_answerable,
        "predicted_answer": predicted_answer,
        "predicted_answerable": predicted_answerable,
        "answer_score": answer_accuracy.score(prediction=response, expected_answer=expected_answer).value,
        "answerable_score": answerable_accuracy.score(prediction=response, expected_answerable=expected_answerable).value,
    }

import os, pandas as pd
from ragas import Dataset
from ragas.backends import InMemoryBackend, LocalCSVBackend

def load_squad():
    cache = getattr(load_squad, "_cache", {})
    if not cache:
        cache = load_dataset("rajpurkar/squad_v2", split="validation[:10]")
        load_squad._cache = cache
    return cache

if __name__ == "__main__":
    async def main() -> None:
        run_id = datetime.datetime.now().strftime("%Y%m%d-%H%M%S")
        exp_name = "squad_experiment"

        current_dir = os.path.dirname(os.path.abspath(__file__))
        squad_rows = list(load_squad())
        dataset = Dataset(
            name="squad_v2_validation_10",
            data=squad_rows,
            backend=InMemoryBackend(),
        )

        exp = await squad_experiment.arun(
            dataset,
            name=f"{run_id}-{exp_name}",
            prompt_file="evals_promptv1.txt",
            experiment_name=exp_name,
            backend=LocalCSVBackend(root_dir=current_dir)
        )

    asyncio.run(main())