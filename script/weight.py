# made with chatgpt
# updates the leaderboard of the biggest fish

import re
import asyncio
import aiohttp
from collections import defaultdict
import csv

# List of URLs containing the fish catch information
urls = [
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

# Function to read cheaters from CSV file
def read_cheaters(filename):
    cheaters = []
    with open('lists/cheaters.csv', 'r') as file:
        reader = csv.reader(file)
        for row in reader:
            cheaters.append(row[0])
    return cheaters

# Function to read verified players from CSV file
def read_verified_players(filename):
    verified_players = []
    with open('lists/verified.csv', 'r') as file:
        reader = csv.reader(file)
        for row in reader:
            verified_players.append(row[0])
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

    # Open and read the existing leaderboard file
    with open('leaderboardweight.txt', 'r', encoding='utf-8') as file:
        for line in file:
            if line.startswith("#"):
                # Extract player name, fish type, and weight from the line
                parts = line.split()
                player = parts[1].rstrip(':')  # Remove the trailing colon if present
                fish_type = parts[2]
                fish_weight = float(parts[3])
                bot = None  # Initialize bot as None by default
                # Check if the marker is present indicating 'supibot'
                if '*' in player:
                    player = player.rstrip('*')
                    bot = 'supibot'
                player = renamed_chatters.get(player, player)
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
    with open('leaderboardweight.txt', 'w', encoding='utf-8') as file:
        file.write("Chatters and their biggest fish caught in chat (>200 lbs):\n")
        for rank, (player, fish_details) in enumerate(sorted(merged_records.items(), key=lambda x: x[1]['weight'], reverse=True), start=1):
            if fish_details['weight'] > 200:
                # Check if the player is not in the verified_players list and caught their fish on "supibot"
                if player not in verified_players and merged_records[player]['bot'] == 'supibot':
                    file.write(f"#{rank} {player}*: {fish_details['type']} {fish_details['weight']} lbs\n")
                else:
                    file.write(f"#{rank} {player}: {fish_details['type']} {fish_details['weight']} lbs\n")
        file.write("* = The fish was caught on supibot and the player did not migrate their data over to gofishgame. Because of that their data was not individually verified to be accurate.\n")

if __name__ == "__main__":
    renamed_chatters = renamed('lists/renamed.csv')
    cheaters = read_cheaters('lists/cheaters.csv')
    verified_players = read_verified_players('lists/verified.csv')
    asyncio.run(main(renamed_chatters, verified_players, cheaters))