# made with chatgpt
# gets the new tournament results and adds them to logs.txt

import os
import urllib.request

class URLSet:
    def __init__(self, urls, logs):
        self.urls = urls
        self.logs = logs

def fetch_matching_lines(set_info):
    log_file_path = set_info.logs if os.path.isabs(set_info.logs) else os.path.join('logs', set_info.logs)
    matching_lines = []

    for url in set_info.urls:
        try:
            with urllib.request.urlopen(url) as response:
                body = response.read().decode('utf-8')
                lines = body.split('\n')
                matching_lines.extend(line.strip() for line in lines if "The results are in" in line
                                       or "The results for last week are in" in line
                                       or "Last week..." in line)
        except Exception as e:
            print(f"Error fetching URL: {e}")

    existing_lines = set()
    if os.path.exists(log_file_path):
        try:
            with open(log_file_path, 'r', encoding='utf-8') as file:
                for line in file:
                    if "You caught" in line:
                        parts = line.split("You caught", 1)
                        if len(parts) > 1:
                            existing_lines.add(parts[1].strip())
        except FileNotFoundError:
            pass  # File doesn't exist, so no existing lines to read
    else:
        os.makedirs(os.path.dirname(log_file_path), exist_ok=True)

    new_results = []
    for line in matching_lines:
        if "You caught" in line:
            parts = line.split("You caught", 1)
            if len(parts) > 1:
                new_line = parts[1].strip()
                if new_line not in existing_lines:
                    new_results.append(line)
                    existing_lines.add(new_line)

    if new_results:
        with open(log_file_path, 'a', encoding='utf-8') as file:
            for line in new_results:
                file.write(line + '\n')
        print(f"New results appended to {log_file_path}")
    else:
        print(f"No new results to append to {log_file_path}")

def main():
    url_set = URLSet(urls=["https://logs.joinuv.com/channel/breadworms/user/gofishgame/2024/3?"], logs="logs.txt")

    fetch_matching_lines(url_set)

if __name__ == "__main__":
    main()
