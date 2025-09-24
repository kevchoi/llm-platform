import ollama

def summarize_text(text: str) -> str:
    response = ollama.chat(
        model="qwen3:0.6b",
        messages=[
            {
                "role": "system",
                "content": "You are a concise summarization assistant.",
            },
            {
                "role": "user",
                "content": f"Summarize the following passage in a few sentences:\n\n{text}",
            },
        ],
    )
    return response["message"]["content"].strip()


if __name__ == "__main__":
    with open("latency_optimization.txt", "r") as file:
        text = file.read()
    print(summarize_text(text))
