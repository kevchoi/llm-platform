from locust import HttpUser, task, between

# for some reason, capping out at 20 RPS even though I have a lot of users? Queue depth is increasing as well.
class VLLMUser(HttpUser):
    wait_time = between(1, 3)

    @task
    def generate_text(self):
        payload = {
            "prompt": "What is deep learning?",
            "max_tokens": 100,
            "temperature": 0.7,
        }

        response = self.client.post(
            "/v1/completions",
            json=payload,
            headers={"Content-Type": "application/json"}
        )
        print(response.json())