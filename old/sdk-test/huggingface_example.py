import torch
from transformers import pipeline


def summarize_text(text: str) -> str:
    generator = pipeline(
        task="text-generation",
        model="Qwen/Qwen3-0.6B",
        dtype=torch.float16,
        device_map={"": "mps"} if torch.backends.mps.is_available() else "auto",
    )

    messages = [
        {"role": "system", "content": "You are a concise summarization assistant."},
        {"role": "user", "content": f"Summarize the following passage in a few sentences:\n\n{text}"},
    ]

    response = generator(
        messages,
        max_new_tokens=1024,
        temperature=1.0,
        return_full_text=False,
    )

    text = response[0]["generated_text"]
    return text


if __name__ == "__main__":
    with open("latency_optimization.txt", "r") as file:
        text = file.read()
    print(summarize_text(text))