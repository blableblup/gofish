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

    old_rankings = {}
    
    # Open and read the existing leaderboard file
    with open('leaderboardtype.md', 'r', encoding='utf-8') as file:
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
                player = parts[4].strip()
                fish_type = parts[2].strip()
                if fish_type in equivalent_fish_types:
                    fish_type = equivalent_fish_types[fish_type]  # Update fish type if it has an equivalent
                fish_weight = float(parts[3].strip().split()[0])
                bot = None  # Initialize bot as None by default
                # Check if the marker is present indicating 'supibot'
                if '*' in player:
                    player = player.rstrip('*')
                    bot = 'supibot'
                player = renamed_chatters.get(player, player)
                old_rankings[fish_type] = int(rank)
                old_record[fish_type] = {'weight': fish_weight, 'player': player, 'bot': bot}

    # Compare new records with old records and update if necessary
    updated_records = {} # Create a new dictionary to store updated records
    for fish_type in list(new_record.keys()):
        if fish_type in old_record: 
            # Update old record with the new record
            if new_record[fish_type]['weight'] > old_record[fish_type]['weight']:
                old_record[fish_type] = {'weight': new_record[fish_type]['weight'], 'player': new_record[fish_type]['player'], 'bot': new_record[fish_type]['bot']}
                updated_records[fish_type] = old_record[fish_type]
        else:
            # Add the new fish
            updated_records[fish_type] = new_record[fish_type]
    
    # Merge old_record and updated_records dictionaries            
    merged_records = {**old_record, **updated_records}

    # Sort fish types based on their weights
    sorted_leaderboard = sorted(merged_records.items(), key=lambda x: x[1].get('weight', 0), reverse=True)

    # Write the updated leaderboard to leaderboardtype.txt
    with open('leaderboardtype.md', 'w', encoding='utf-8') as file:
        file.write("### Leaderboard for the biggest fish per type caught in chat\n\n")
        file.write("| Rank | Fish Type | Weight | Player |\n")
        file.write("|------|-----------|--------|--------|\n")
        rank = 1
        for fish_type, fish_info in sorted_leaderboard:
            # Ranking change
            movement = {}
            old_rank = old_rankings.get(fish_type)
            if old_rank:
                if rank < old_rank:
                    movement[fish_type] = '⬆'
                elif rank > old_rank:
                    movement[fish_type] = '⬇'
                else:
                    movement[fish_type] = ''
            else:
                movement[fish_type] = '🆕'
            if fish_info['player'] not in verified_players and fish_info['bot'] == 'supibot':
                file.write(f"| {rank} {movement[fish_type]}| {fish_type} | {fish_info['weight']} lbs | {fish_info['player']}* |\n")
            else:
                file.write(f"| {rank} {movement[fish_type]}| {fish_type} | {fish_info['weight']} lbs | {fish_info['player']} |\n")
            rank += 1
        file.write("\n_* = The fish was caught on supibot and the player did not migrate their data over to gofishgame. Because of that their data was not individually verified to be accurate._\n")

if __name__ == "__main__":
    renamed_chatters = renamed('lists/renamed.csv')
    cheaters = read_cheaters('lists/cheaters.txt')
    verified_players = read_verified_players('lists/verified.txt')
    asyncio.run(main(renamed_chatters, cheaters, verified_players))
