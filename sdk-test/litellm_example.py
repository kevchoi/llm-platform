from litellm import completion


def summarize_text(text: str) -> str:
    response = completion(
        model="gpt-5-nano-2025-08-07",
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
        max_tokens=1024,
    )
    return response.choices[0].message.content.strip()


if __name__ == "__main__":
    with open("latency_optimization.txt", "r") as file:
        text = file.read()
    print(summarize_text(text))