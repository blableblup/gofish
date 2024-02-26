# made with chatgpt
# finds the biggest fish per fish type and who caught it and updates the leaderboard
# jellyfish is a bttv emote currently

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

# Define a mapping for equivalent fish types
equivalent_fish_types = {
    '🕷': '🕷️', '🗡' : '🗡️', '🕶' : '🕶️', '☂' : '☂️', '⛸' : '⛸️', '🧜♀' : '🧜‍♀️', '🧜♀️' : '🧜‍♀️', '🧜‍♀' : '🧜‍♀️', '🐻‍❄️' : '🐻‍❄', '🧞‍♂️' : '🧞‍♂', 'HailHelix' : '🐚',
}

# Define a dictionary to store the biggest fish of each fish type
new_record = defaultdict(lambda: {'player': '', 'weight': 0, 'bot': None})

# Define a regex pattern to extract information about fish catches
pattern = r"\s?(\w+): [@👥]\s?(\w+), You caught a [✨🫧] (.*?) [✨🫧]! It weighs ([\d.]+) lbs"

async def fetch_data(url, renamed_chatters, cheaters):
    async with aiohttp.ClientSession() as session:
        async with session.get(url) as response:
            text_content = await response.text()
            # Extract information about fish catches from the text content
            for match in re.finditer(pattern, text_content):
                bot, player, fish_type, fish_weight_str = match.groups()
                weight = float(fish_weight_str)
                # Check if the player name has a mapping to a new name
                player = renamed_chatters.get(player, player)
                if player in cheaters:
                    continue  # Skip processing for ignored players 
                if fish_type in equivalent_fish_types:
                    fish_type = equivalent_fish_types[fish_type]  # Update fish type if it has an equivalent
                # Update the biggest fish of each fish type
                if weight > new_record[fish_type]['weight']:
                    new_record[fish_type] = {'player': player, 'weight': weight, 'bot': bot}

async def main(renamed_chatters, cheaters, verified_players):
    for url in urls:
        await fetch_data(url, renamed_chatters, cheaters)

    # Initialize old_record with default values
    old_record = defaultdict(lambda: {'weight': 0, 'player': '', 'bot': None})

    # Open and read the existing leaderboard file
    with open('leaderboardtype.txt', 'r', encoding='utf-8') as file:
        for line in file:
            if line.startswith("#"):
                # Extract player name, fish type, and weight from the line
                parts = line.split()
                player = parts[4] 
                fish_type = parts[1]
                weight = float(parts[2])
                bot = None  # Initialize bot as None by default
                # Check if the marker is present indicating 'supibot'
                if '*' in player:
                    player = player.rstrip('*')
                    bot = 'supibot'
                player = renamed_chatters.get(player, player)
                old_record[fish_type] = {'weight': weight, 'player': player, 'bot': bot}

    # Compare fetched data with existing data and update the leaderboard if necessary
    updated_leaderboard = {}
    for fish_type in list(new_record.keys()):
        if fish_type in old_record: 
            if new_record[fish_type]['weight'] > old_record[fish_type]['weight']:
                old_record[fish_type] = {'weight': new_record[fish_type]['weight'], 'player': new_record[fish_type]['player'], 'bot': new_record[fish_type]['bot']}
                updated_leaderboard[fish_type] = old_record[fish_type]
        else:
            updated_leaderboard[fish_type] = new_record[fish_type]
            
    merged_records = {**old_record, **updated_leaderboard}

    # Sort fish types based on their weights
    sorted_leaderboard = sorted(merged_records.items(), key=lambda x: x[1].get('weight', 0), reverse=True)

    # Write the updated leaderboard to leaderboardtype.txt
    with open('leaderboardtype.txt', 'w', encoding='utf-8') as file:
        file.write("Biggest fish by type caught in chat:\n")
        rank = 1
        for fish_type, fish_info in sorted_leaderboard:
            if fish_info['player'] not in verified_players and fish_info['bot'] == 'supibot':
                file.write(f"#{rank} {fish_type} {fish_info['weight']} lbs, {fish_info['player']}* \n")
            else:
                file.write(f"#{rank} {fish_type} {fish_info['weight']} lbs, {fish_info['player']} \n")
            rank += 1
        file.write("* = The fish was caught on supibot and the player did not migrate their data over to gofishgame. Because of that their data was not individually verified to be accurate.\n")

if __name__ == "__main__":
    renamed_chatters = renamed('lists/renamed.csv')
    cheaters = read_cheaters('lists/cheaters.csv')
    verified_players = read_verified_players('lists/verified.csv')
    asyncio.run(main(renamed_chatters, cheaters, verified_players))
