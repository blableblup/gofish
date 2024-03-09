# made with chatgpt
# updates the leaderboard of the biggest fish

import re
import asyncio
import aiohttp
from collections import defaultdict
import csv

# List of URLs containing the fish catch information
urls = [
    'https://logs.joinuv.com/channel/breadworms/user/gofishgame/2024/3?',
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
            # Append the line to the list of verified players
            verified_players.append(line.strip())  # Strip any leading or trailing whitespace
    return verified_players

# Define a dictionary to store the details of the biggest fish caught by each player
new_record = defaultdict(lambda: {'weight': 0, 'type': None, 'bot': None})

# Define a regex pattern to extract information about fish catches
pattern = r"\s?(\w+): [@👥]\s?(\w+), You caught a [✨🫧] (.*?) [✨🫧]! It weighs ([\d.]+) lbs"

async def fetch_data(url, renamed_chatters, cheaters):
   
    async with aiohttp.ClientSession() as session:
        async with session.get(url) as response:
            text_content = await response.text()
            # Extract information about fish catches from the text content
            for match in re.finditer(pattern, text_content):
                bot, player, fish_type, fish_weight_str = match.groups()
                if player in cheaters:
                    continue  # Skip processing for ignored players
                # Check if the player name has a mapping to a new name
                player = renamed_chatters.get(player, player)
                fish_weight = float(fish_weight_str)
                # Store the bot where the fish was caught
                if fish_weight > new_record[player]['weight']:
                    new_record[player] = {'weight': fish_weight, 'type': fish_type, 'bot': bot}

async def main(renamed_chatters, cheaters, verified_players):
    for url in urls:
        await fetch_data(url, renamed_chatters, cheaters)
    
    # Initialize old_record with default values
    old_record = defaultdict(lambda: {'weight': 0, 'type': None, 'bot': None})

    old_rankings = {}

    # Open and read the existing leaderboard file
    with open('leaderboardweight.md', 'r', encoding='utf-8') as file:
        next(file)  # Skip first 4 lines
        next(file)  
        next(file)
        next(file)
        for line in file:
            if line.startswith("|"):
                # Extract rank, player name, fish type, and weight from the line
                parts = line.split("|")
                rank = parts[1].strip()
                rank = rank.split()[0]
                player = parts[2].strip()
                fish_type = parts[3].strip()
                fish_weight = float(parts[4].strip().split()[0])
                bot = None  # Initialize bot as None by default
                # Check if the marker is present indicating 'supibot'
                if '*' in player:
                    player = player.rstrip('*')
                    bot = 'supibot'
                player = renamed_chatters.get(player, player)
                old_rankings[player] = int(rank)
                old_record[player] = {'weight': fish_weight, 'type': fish_type, 'bot': bot}

    # Compare new records with old records and update if necessary
    updated_records = {}  # Create a new dictionary to store updated records
    for player in new_record.keys():
        if new_record[player]['weight'] > old_record[player]['weight']:
            # Update old record with the new record
            old_record[player] = {'weight': new_record[player]['weight'], 'type': new_record[player]['type'], 'bot': new_record[player]['bot']}
            updated_records[player] = old_record[player]

    # Merge old_record and updated_records dictionaries
    merged_records = {**old_record, **updated_records}

    # Write the updated records to the leaderboard file
    with open('leaderboardweight.md', 'w', encoding='utf-8') as file:
        file.write("### Leaderboard for the biggest fish caught per player in chat\n\n")
        file.write("| Rank | Player | Fish | Weight ⚖️ |\n")
        file.write("|------|--------|-----------|---------|\n")
        for rank, (player, fish_info) in enumerate(sorted(merged_records.items(), key=lambda x: x[1]['weight'], reverse=True), start=1):
            if fish_info['weight'] > 200:
                # Ranking change
                movement = {}
                old_rank = old_rankings.get(player)
                if old_rank:
                    if rank < old_rank:
                        movement[player] = '⬆'
                    elif rank > old_rank:
                        movement[player] = '⬇'
                    else:
                        movement[player] = ''
                else:
                    movement[player] = '🆕'
                # Check if the player is not in the verified_players list and caught their fish on "supibot"
                if player not in verified_players and merged_records[player]['bot'] == 'supibot':
                    file.write(f"| {rank} {movement[player]}| {player}* | {fish_info['type']} | {fish_info['weight']} lbs |\n")
                else:
                    file.write(f"| {rank} {movement[player]}| {player} | {fish_info['type']} | {fish_info['weight']} lbs |\n")
        file.write("\n_* = The fish was caught on supibot and the player did not migrate their data over to gofishgame. Because of that their data was not individually verified to be accurate._\n")

if __name__ == "__main__":
    renamed_chatters = renamed('lists/renamed.csv')
    cheaters = read_cheaters('lists/cheaters.txt')
    verified_players = read_verified_players('lists/verified.txt')
    asyncio.run(main(renamed_chatters, cheaters, verified_players))
