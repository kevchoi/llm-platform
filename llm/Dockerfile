FROM ghcr.io/ggml-org/llama.cpp:server

ARG MODEL_URL="https://huggingface.co/Qwen/Qwen3-0.6B-GGUF/resolve/main/Qwen3-0.6B-Q8_0.gguf"

RUN mkdir -p /app/models
ADD ${MODEL_URL} /app/models/model.gguf

CMD ["-m", "/app/models/model.gguf", "--port", "8080", "--host", "0.0.0.0"]