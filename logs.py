# made with chatgpt
# gets the tournament results and adds them to logs.txt
# have to manually delete people who checkin multiple times and older results

import requests
from concurrent.futures import ThreadPoolExecutor

# URLs of the websites containing the results
urls = [
        'https://logs.joinuv.com/channel/breadworms/user/gofishgame/2024/2?',
]

# Function to fetch matching lines from a single URL
def fetch_matching_lines(url):
    response = requests.get(url)
    text_content = response.text
    lines = text_content.split('\n')
    matching_lines = [line.strip() for line in lines if 'The results are in' in line or 'The results for last week are in' in line or 'Last week...' in line]
    return matching_lines

# Fetch matching lines from multiple URLs concurrently
with ThreadPoolExecutor() as executor:
    # Submit tasks for each URL
    future_to_url = {executor.submit(fetch_matching_lines, url): url for url in urls}
    for future in future_to_url:
        url = future_to_url[future]
        try:
            matching_lines = future.result()
            # Write matching lines to a single file
            with open('logs.txt', 'a', encoding='utf-8') as txt_file:
                for line in matching_lines:
                    txt_file.write(line + '\n')
            print(f"Processed {url}")
        except Exception as e:
            print(f"Error processing {url}: {e}")
