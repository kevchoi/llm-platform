Add the hugging face secret:

kubectl create secret generic hf-token --from-literal=hf_token='hf_XXXXXXX' --namespace=ray-service

To run the Ray service: