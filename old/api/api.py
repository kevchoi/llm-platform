from fastapi import FastAPI
from fastapi.responses import StreamingResponse
from typing import List
from pydantic import BaseModel
import logging
import os
import httpx

logging.basicConfig(level=logging.INFO)
logger = logging.getLogger(__name__)
app = FastAPI()


LLAMA_CPP_URL = os.getenv("LLAMA_CPP_URL")

class ChatMessage(BaseModel):
    role: str
    content: str

class ChatCompletionsRequest(BaseModel):
    model: str = "qwen3-0.6b-gguf"
    messages: List[ChatMessage]
    max_completion_tokens: int | None = None
    stream: bool = False
    # temperature: float = 1.0
    # top_p: float = 1.0

class ChatCompletionChoice(BaseModel):
    index: int
    message: ChatMessage
    finish_reason: str

class ChatCompletionsResponse(BaseModel):
    model: str
    id: str
    object: str
    created: int
    choices: List[ChatCompletionChoice]
    # usage: ChatCompletionUsage

@app.post("/v1/chat/completions")
async def chat_completion(request: ChatCompletionsRequest) -> ChatCompletionsResponse:
    payload = {
        "messages": [{"role": msg.role, "content": msg.content} for msg in request.messages],
        "max_tokens": request.max_completion_tokens,
        "stream": request.stream,
    }

    async with httpx.AsyncClient() as client:
        response = await client.post(
            f"{LLAMA_CPP_URL}/v1/chat/completions",
            json=payload,
        )
        if request.stream:
            async def generate():
                async for line in response.aiter_lines():
                    if line:
                        yield f"{line}\n\n"
            return StreamingResponse(generate(), media_type="text/event-stream")

        data = response.json()
        logger.info("response: %s", data)
        return ChatCompletionsResponse(
            id=data["id"],
            model=data["model"],
            object=data["object"],
            created=data["created"],
            choices=[
                ChatCompletionChoice(
                    index=c["index"],
                    message=ChatMessage(
                        role=c["message"]["role"],
                        content=c["message"]["content"],
                    ),
                    finish_reason=c["finish_reason"],
                )
                for c in data["choices"]
            ],
        )