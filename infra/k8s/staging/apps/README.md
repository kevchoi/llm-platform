Add the hugging face secret:

kubectl create secret generic hf-token --from-literal=hf_token='YOUR_HUGGING_FACE_TOKEN'

To run the Ray service:

