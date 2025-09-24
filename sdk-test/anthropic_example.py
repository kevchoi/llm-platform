from anthropic import Anthropic

client = Anthropic()

def summarize_text(text: str) -> str:
    response = client.messages.create(
        model="claude-sonnet-4-20250514",
        max_tokens=1024,
        system="You are a concise summarization assistant.",
        messages=[
            {
                "role": "user",
                "content": f"Summarize the following passage in a few sentences:\n\n{text}"
            }
        ],
    )
    return response.content

if __name__ == "__main__":
    with open("latency_optimization.txt", "r") as file:
        text = file.read()
    print(summarize_text(text))