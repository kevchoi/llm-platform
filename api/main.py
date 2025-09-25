from fastapi import FastAPI
from typing import List
from pydantic import BaseModel
from openai import OpenAI


app = FastAPI()
client = OpenAI()

class ChatMessage(BaseModel):
    role: str
    content: str


class ChatCompletionRequest(BaseModel):
    model: str
    messages: List[ChatMessage]
    max_tokens: int | None = None

@app.post("/v1/chat/completions")
async def chat_completion(request: ChatCompletionRequest):
    response = client.responses.create(
        model=request.model,
        input=request.messages,
        max_output_tokens=request.max_tokens,
    )
    return response.output_text.strip()