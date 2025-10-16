from locust import HttpUser, task, between

# ran to about 350 RPS; saw failures at 25 RPS
class TGIUser(HttpUser):
    wait_time = between(1, 3)

    @task
    def generate_text(self):
        payload = {
            "inputs": "What is deep learning?",
            "parameters": {
                "max_new_tokens": 100,
                "temperature": 0.7,
            }
        }
        
        response = self.client.post(
            "/generate",
            json=payload,
            headers={"Content-Type": "application/json"}
        )
        print(response.json())