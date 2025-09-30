from google import genai
import os

client = genai.Client()

def summarize_text(text: str) -> str:
    response = client.models.generate_content(
        model="gemini-2.5-flash-lite",
        config=genai.types.GenerateContentConfig(
            system_instruction="You are a concise summarization assistant.",
        ),
        contents=f"Summarize the following passage in a few sentences:\n\n{text}",
    )
    return response.text

if __name__ == "__main__":
    with open("latency_optimization.txt", "r") as file:
        text = file.read()
    print(summarize_text(text))