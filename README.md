To run:

```
docker compose build --no-cache api llm
docker compose up --build
```

To test:

```
curl -X POST http://localhost:8000/v1/chat/completions \
-H "Content-Type: application/json" \
-d '{"messages":[{"role":"user","content":"Hello!"}],"stream":false}'
```