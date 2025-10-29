Add the hugging face secret:

kubectl create secret generic hf-token --from-literal=hf_token='hf_XXXXXXX' --namespace=ray-service

Debugging Ray Service:

```
kubectl port-forward svc/ray-serve-llm-hck22-head-svc 8265 -n ray-service
```

```
kubectl get rayservice ray-serve-llm -o yaml -n ray-service
kubectl get svc -n ray-service

kubectl port-forward svc/ray-serve-llm-serve-svc 8000:8000 -n ray-service

curl --location 'http://localhost:8000/v1/chat/completions' \
  --header 'Content-Type: application/json' \
  --data '{
      "model": "Qwen/Qwen3-32B-AWQ",
      "messages": [
          {
              "role": "system", 
              "content": "You are a helpful assistant."
          },
          {
              "role": "user", 
              "content": "Provide steps to serve an LLM using Ray Serve."
          }
      ]
  }'
```

Debugging:
```
# List ray worker pods
kubectl get pods -n ray-service -l ray.io/node-type=worker

# Exec into a worker
kubectl exec -it <ray-worker-pod-name> -n ray-service -- /bin/bash

# Then run your script or debugging commands
python your_script.py

# List ray worker pods
kubectl get pods -n ray-service -l ray.io/node-type=worker

# Exec into a worker
kubectl exec -it <ray-worker-pod-name> -n ray-service -- /bin/bash

# Then run your script or debugging commands
python your_script.py
```