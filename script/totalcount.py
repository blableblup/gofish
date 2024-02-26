# made with chatgpt
# counts how many fish someone caught in chat and ranks them
# doesnt consider "glimmering", "was in its mouth" and "something jumped out of the water to snatch your rare candy" as catches right now
# has to check the logs from the beginning 

import re
import asyncio
import aiohttp
from collections import defaultdict
import csv

# List of URLs containing the fish catch information
urls = [
    'https://logs.joinuv.com/channel/breadworms/user/supibot/2022/12?',
    'https://logs.joinuv.com/channel/breadworms/user/supibot/2023/1?',
    'https://logs.joinuv.com/channel/breadworms/user/supibot/2023/2?',
    'https://logs.joinuv.com/channel/breadworms/user/supibot/2023/3?',
    'https://logs.joinuv.com/channel/breadworms/user/supibot/2023/4?',
    'https://logs.joinuv.com/channel/breadworms/user/supibot/2023/5?',
    'https://logs.joinuv.com/channel/breadworms/user/supibot/2023/6?',
    'https://logs.joinuv.com/channel/breadworms/user/supibot/2023/7?',
    'https://logs.joinuv.com/channel/breadworms/user/supibot/2023/8?',
    'https://logs.joinuv.com/channel/breadworms/user/supibot/2023/9?',
    'https://logs.joinuv.com/channel/breadworms/user/gofishgame/2023/9?',
    'https://logs.joinuv.com/channel/breadworms/user/gofishgame/2023/10?',
    'https://logs.joinuv.com/channel/breadworms/user/gofishgame/2023/11?',
    'https://logs.joinuv.com/channel/breadworms/user/gofishgame/2023/12?',
    'https://logs.joinuv.com/channel/breadworms/user/gofishgame/2024/1?',
    'https://logs.joinuv.com/channel/breadworms/user/gofishgame/2024/2?',
    # Add more URLs as needed
]

# Function to read renamed chatters from CSV file
def renamed(filename):
    renamed_chatters = {}
    with open('lists/renamed.csv', 'r') as file:
        reader = csv.DictReader(file)
        for row in reader:
            old_player = row['old_name']
            new_player = row['new_name']
            renamed_chatters[old_player] = new_player
    return renamed_chatters

# Function to read cheaters from a text file
def read_cheaters(filename):
    cheaters = []
    with open('lists/cheaters.txt', 'r') as file:
        # Read each line from the text file
        for line in file:
            # Append the line to the list of cheaters
            cheaters.append(line.strip())  # Strip any leading or trailing whitespace
    return cheaters

# Function to read verified players from a text file
def read_verified_players(filename):
    verified_players = []
    with open('lists/verified.txt', 'r') as file:
        # Read each line from the text file
        for line in file:
            # Append the line to the list of cheaters
            verified_players.append(line.strip())  # Strip any leading or trailing whitespace
    return verified_players

# Define a dictionary to store the count of fish caught by each player
fish_catch_count = defaultdict(int)

# Define a regex pattern to extract information about fish catches
pattern = r"\s?(\w+): [@👥]\s?(\w+), You caught a [✨🫧] (.*?) [✨🫧]!"

# Keep track of players who caught their first fish with supibot
first_catch_with_supibot = defaultdict(bool)

async def fetch_data(url, renamed_chatters, cheaters, verified_players):
    async with aiohttp.ClientSession() as session:
        async with session.get(url) as response:
            text_content = await response.text()
            # Extract information about fish catches from the text content
            for match in re.finditer(pattern, text_content):
                player_name = match.group(2)
                bot_name = match.group(1)
                # Check if the player is in the ignore list
                if player_name in cheaters:
                    continue
                # Check if the player name has a mapping to a new name
                player_name = renamed_chatters.get(player_name, player_name)
                # Add a verification check
                if bot_name == "supibot" and player_name not in verified_players and not first_catch_with_supibot[player_name]:
                    first_catch_with_supibot[player_name] = True
                # Update the fish catch count for the player
                fish_catch_count[player_name] += 1

async def main(renamed_chatters, cheaters, verified_players):
    for url in urls:
        await fetch_data(url, renamed_chatters, cheaters, verified_players)
    
    global fish_catch_count

    # Filter players who caught more than 100 fish
    fish_catch_count = {player: count for player, count in fish_catch_count.items() if count > 100}

    # Rank the players based on the number of fish they caught
    ranked_players = sorted(fish_catch_count.items(), key=lambda x: x[1], reverse=True)

    # Assign ranks to players
    ranks = defaultdict(list)
    rank = 1
    for player, count in ranked_players:
        ranks[count].append(player)

    # Write the results to a text file
    with open('leaderboardtotalcount.txt', 'w', encoding='utf-8') as file:
        file.write("The most fish caught in chat (since December 2022):\n")
        for count, players in ranks.items():
            for player in players:
                if first_catch_with_supibot[player] == True:
                    file.write(f"#{rank} {player}*: {count} fish caught \n")
                else:
                    file.write(f"#{rank} {player}: {count} fish caught \n")
            rank += len(players)
        file.write("* = The player caught their first fish on supibot and did not migrate their data to gofishgame. Because of that their data was not individually verified to be accurate.\n")

if __name__ == "__main__":
    renamed_chatters = renamed('lists/renamed.csv')
    cheaters = read_cheaters('lists/cheaters.txt')
    verified_players = read_verified_players('lists/verified.txt')
    asyncio.run(main(renamed_chatters, cheaters, verified_players))
