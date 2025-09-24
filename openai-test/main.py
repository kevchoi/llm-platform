from openai import OpenAI

client = OpenAI()

def summarize_text(text: str) -> str:
    response = client.responses.create(
        model="gpt-5-nano-2025-08-07",
        input=[
            {
                "role": "system",
                "content": "You are a concise summarization assistant."
            },
            {
                "role": "user",
                "content": f"Summarize the following passage in a few sentences:\n\n{text}"
            },
        ],
        max_output_tokens=1000,
    )
    return response.output_text.strip()

if __name__ == "__main__":
    with open("latency_optimization.txt", "r") as file:
        text = file.read()
    print(summarize_text(text))