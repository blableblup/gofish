# made with chatgpt
# gets new tournament results and adds them to logs.txt
# still have to delete new lines if chatter does +checkin multiple times

import asyncio
import requests
from concurrent.futures import ThreadPoolExecutor

# URLs of the websites containing the results
urls = [
        'https://logs.joinuv.com/channel/breadworms/user/gofishgame/2024/2?',
]

# Function to fetch matching lines from a single URL
async def fetch_matching_lines(url):
    response = requests.get(url)
    text_content = response.text
    lines = text_content.split('\n')
    matching_lines = [line.strip() for line in lines if 'The results are in' in line or 'The results for last week are in' in line or 'Last week...' in line]
    return matching_lines

async def main():
    # Fetch matching lines from multiple URLs concurrently
    matching_lines_list = await asyncio.gather(*[fetch_matching_lines(url) for url in urls])
    all_matching_lines = [line for lines in matching_lines_list for line in lines]

    # Read existing content from logs.txt
    with open('logs/logs.txt', 'r', encoding='utf-8') as file:
        existing_content = file.readlines()

    # Extract relevant parts from existing content
    existing_lines = set()
    for line in existing_content:
        if 'You caught' in line:
            parts = line.split('You caught', 1)
            if len(parts) > 1:
                existing_lines.add(parts[1].strip())

    # Extract and compare new results to ensure uniqueness
    new_results = set()
    for line in all_matching_lines:
        if 'You caught' in line:
            parts = line.split('You caught', 1)
            if len(parts) > 1:
                new_line = parts[1].strip()
                if new_line not in existing_lines:
                    new_results.add(line)  # Add the entire line, not just the stripped part
    
    # Append only the unique new results to logs.txt
    if new_results:
        with open('logs/logs.txt', 'a', encoding='utf-8') as file:
            for line in new_results:
                file.write(line + '\n')
        print("New results appended")
    else:
        print("No new results to append")

if __name__ == "__main__":
    asyncio.run(main())