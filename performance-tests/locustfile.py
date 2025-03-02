from locust import HttpUser, TaskSet, task, between

class UserBehavior(TaskSet):
    def on_start(self):
        # Define headers with the API key
        self.headers = {
            "X-API-Key": "endpoint-test",
        }

    @task
    def get_resource(self):
        # Pass headers in the request
        self.client.get("http://localhost:9090/api/v2/actions/notion/oauth/auth-session", headers=self.headers)

class WebsiteUser(HttpUser):
    tasks = [UserBehavior]
    wait_time = between(1, 3)

