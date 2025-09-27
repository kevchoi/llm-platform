To run on Apple Silicon:

```
docker build --platform linux/amd64 -t llm:latest /Users/kev/code/llm-platform/llm
docker run --rm -p 8000:8000 llm
```