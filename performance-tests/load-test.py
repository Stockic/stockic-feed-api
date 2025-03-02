import requests
import time
import matplotlib.pyplot as plt

API_URL = "http://api.adityapatil.dev/api/v1/headlines/1/1"
CONCURRENT_USERS = 1000
REQUESTS_PER_USER = 30
HEADERS = {
    "X-API-Key": "raw-testing",
}

def send_requests():
    print("[+] Sending Requests")
    response_times = []
    errors = 0

    for _ in range(REQUESTS_PER_USER):
        start_time = time.time()
        try:
            response = requests.get(API_URL, headers=HEADERS)
            if response.status_code != 200:
                errors += 1
        except requests.exceptions.RequestException:
            errors += 1
        finally:
            response_times.append(time.time() - start_time)

    return response_times, errors

def main():
    import concurrent.futures
    response_times = []
    total_errors = 0

    with concurrent.futures.ThreadPoolExecutor(max_workers=CONCURRENT_USERS) as executor:
        futures = [executor.submit(send_requests) for _ in range(CONCURRENT_USERS)]
        for future in concurrent.futures.as_completed(futures):
            times, errors = future.result()
            response_times.extend(times)
            total_errors += errors

    avg_response_time = sum(response_times) / len(response_times)
    max_response_time = max(response_times)
    min_response_time = min(response_times)

    print(f"Average Response Time: {avg_response_time:.2f} seconds")
    print(f"Max Response Time: {max_response_time:.2f} seconds")
    print(f"Min Response Time: {min_response_time:.2f} seconds")
    print(f"Total Errors: {total_errors}")

    # Plot Response Times
    plt.hist(response_times, bins=50, color='blue', alpha=0.7)
    plt.title("API Response Times")
    plt.xlabel("Response Time (seconds)")
    plt.ylabel("Frequency")
    plt.show()

if __name__ == "__main__":
    main()
