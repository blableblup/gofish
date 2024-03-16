# originally made with chatgpt
# updates the two leaderboards for the players biggest fish and the biggest fish per fish type

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

# Define two dictionaries for the new records
new_record_weight = defaultdict(lambda: {'type': '', 'weight': 0, 'bot': None})
new_record_type = defaultdict(lambda: {'player': '', 'weight': 0, 'bot': None})

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
                
                if player in cheaters:
                    continue  # Skip processing for ignored players
                
                # Change to the latest name
                player = renamed_chatters.get(player, player)
                while player in renamed_chatters:
                    player = renamed_chatters[player]
                
                # Update fish type if it has an equivalent
                if fish_type in equivalent_fish_types:
                    fish_type = equivalent_fish_types[fish_type] 
                
                # Update the record for the biggest fish of the player
                if weight > new_record_weight[player]['weight']:
                    new_record_weight[player] = {'weight': weight, 'type': fish_type, 'bot': bot}
                
                # Update the record for the biggest fish for that type of fish
                if weight > new_record_type[fish_type]['weight']:
                    new_record_type[fish_type] = {'player': player, 'weight': weight, 'bot': bot}


async def main(renamed_chatters, cheaters, verified_players):
    for url in urls:
        await fetch_data(url, renamed_chatters, cheaters)

    # Initialize old_leaderboard_weight with default values for leaderboard of biggest fish per player
    old_leaderboard_weight = defaultdict(lambda: {'weight': 0, 'type': None, 'bot': None})

    # Initialize old_leaderboard_type with default values for leaderboard of biggest fish per type
    old_leaderboard_type = defaultdict(lambda: {'weight': 0, 'player': '', 'bot': None})

    old_rankings_weight = {}
    old_rankings_type = {}

    # Open and read the existing leaderboard files
    with open('leaderboardweight.md', 'r', encoding='utf-8') as file_weight, open('leaderboardtype.md', 'r', encoding='utf-8') as file_type:
        # Skip first 4 lines (header) for both files
        for _ in range(4):
            next(file_weight)
            next(file_type)
        
        for line in file_weight:
            if line.startswith("|"):
                # Extract rank, player name, fish type, and weight from the line
                parts = line.split("|")
                rank = parts[1].strip()
                rank = rank.split()[0]
                player = parts[2].strip()
                fish_type = parts[3].strip()
                if fish_type in equivalent_fish_types:
                    fish_type = equivalent_fish_types[fish_type]  # Update fish type if it has an equivalent
                fish_weight = float(parts[4].strip().split()[0])
                bot = None  
                
                # Check if the marker is present for supibot
                if '*' in player:
                    player = player.rstrip('*')
                    bot = 'supibot'
                
                # Change to the latest name
                player = renamed_chatters.get(player, player)
                while player in renamed_chatters:
                    player = renamed_chatters[player]
                
                old_rankings_weight[player] = int(rank)
                old_leaderboard_weight[player] = {'weight': fish_weight, 'type': fish_type, 'bot': bot}
       
        for line in file_type:
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
                bot = None  
                
                # Check if the marker is present for supibot
                if '*' in player:
                    player = player.rstrip('*')
                    bot = 'supibot'
                
                # Change to the latest name
                player = renamed_chatters.get(player, player)
                while player in renamed_chatters:
                    player = renamed_chatters[player]
                
                old_rankings_type[fish_type] = int(rank)
                old_leaderboard_type[fish_type] = {'weight': fish_weight, 'player': player, 'bot': bot}

    # Compare new records with old records and update if necessary for leaderboard of biggest fish
    updated_records_weight = {}  # Create a new dictionary to store updated records
    for player in new_record_weight.keys():
        if new_record_weight[player]['weight'] > old_leaderboard_weight[player]['weight']:
            old_leaderboard_weight[player] = {'weight': new_record_weight[player]['weight'], 'type': new_record_weight[player]['type'], 'bot': new_record_weight[player]['bot']}
            updated_records_weight[player] = old_leaderboard_weight[player]

    # Compare new records with old records and update if necessary for leaderboard of biggest fish per type
    updated_records_type = {}  # Create a new dictionary to store updated records
    for fish_type in list(new_record_type.keys()):
        if fish_type in old_leaderboard_type: 
            # Update old record with the new record
            if new_record_type[fish_type]['weight'] > old_leaderboard_type[fish_type]['weight']:
                old_leaderboard_type[fish_type] = {'weight': new_record_type[fish_type]['weight'], 'player': new_record_type[fish_type]['player'], 'bot': new_record_type[fish_type]['bot']}
                updated_records_type[fish_type] = old_leaderboard_type[fish_type]
        else:
            # Add the new fish
            updated_records_type[fish_type] = new_record_type[fish_type]
    
    # Merge old_record and updated_records dictionaries for leaderboard of biggest fish
    merged_records_weight = {**old_leaderboard_weight, **updated_records_weight}

    # Merge old_record and updated_records dictionaries for leaderboard of biggest fish per type
    merged_records_type = {**old_leaderboard_type, **updated_records_type}

    # Sort fish based on their weights for leaderboard of biggest fish
    sorted_leaderboard_weight = sorted(merged_records_weight.items(), key=lambda x: x[1]['weight'], reverse=True)

    # Sort fish types based on their weights for leaderboard of biggest fish per type
    sorted_leaderboard_type = sorted(merged_records_type.items(), key=lambda x: x[1]['weight'], reverse=True)

    # Write the updated leaderboard to leaderboardweight.md
    with open('leaderboardweight.md', 'w', encoding='utf-8') as file_weight:
        file_weight.write("### Leaderboard for the biggest fish caught per player in chat\n\n")
        file_weight.write("| Rank | Player | Fish | Weight ⚖️ |\n")
        file_weight.write("|------|--------|-----------|---------|\n")
        rank_weight = 1
        for player, fish_info in sorted_leaderboard_weight:
            if fish_info['weight'] > 200:
                
                # Ranking change
                movement_weight = {}
                old_rank_weight = old_rankings_weight.get(player)
                if old_rank_weight:
                    if rank_weight < old_rank_weight:
                        movement_weight[player] = '⬆'
                    elif rank_weight > old_rank_weight:
                        movement_weight[player] = '⬇'
                    else:
                        movement_weight[player] = ''
                else:
                    movement_weight[player] = '🆕'

                # Check if the player is not in the verified_players list and caught their fish on "supibot"
                if player not in verified_players and fish_info['bot'] == 'supibot':
                    file_weight.write(f"| {rank_weight} {movement_weight[player]}| {player}* | {fish_info['type']} | {fish_info['weight']} lbs |\n")
                else:
                    file_weight.write(f"| {rank_weight} {movement_weight[player]}| {player} | {fish_info['type']} | {fish_info['weight']} lbs |\n")
                rank_weight += 1
        file_weight.write("\n_* = The fish was caught on supibot and the player did not migrate their data over to gofishgame. Because of that their data was not individually verified to be accurate._\n")

    # Write the updated leaderboard to leaderboardtype.md
    with open('leaderboardtype.md', 'w', encoding='utf-8') as file_type:
        file_type.write("### Leaderboard for the biggest fish per type caught in chat\n\n")
        file_type.write("| Rank | Fish Type | Weight | Player |\n")
        file_type.write("|------|-----------|--------|--------|\n")
        rank_type = 1
        for fish_type, fish_info in sorted_leaderboard_type:
            
            # Ranking change
            movement_type = {}
            old_rank_type = old_rankings_type.get(fish_type)
            if old_rank_type:
                if rank_type < old_rank_type:
                    movement_type[fish_type] = '⬆'
                elif rank_type > old_rank_type:
                    movement_type[fish_type] = '⬇'
                else:
                    movement_type[fish_type] = ''
            else:
                movement_type[fish_type] = '🆕'
            
            # Check if the player is not in the verified_players list and caught their fish on "supibot"
            if fish_info['player'] not in verified_players and fish_info['bot'] == 'supibot':
                file_type.write(f"| {rank_type} {movement_type[fish_type]}| {fish_type} | {fish_info['weight']} lbs | {fish_info['player']}* |\n")
            else:
                file_type.write(f"| {rank_type} {movement_type[fish_type]}| {fish_type} | {fish_info['weight']} lbs | {fish_info['player']} |\n")
            rank_type += 1
        file_type.write("\n_* = The fish was caught on supibot and the player did not migrate their data over to gofishgame. Because of that their data was not individually verified to be accurate._\n")

if __name__ == "__main__":
    renamed_chatters = renamed('lists/renamed.csv')
    cheaters = read_cheaters('lists/cheaters.txt')
    verified_players = read_verified_players('lists/verified.txt')
    asyncio.run(main(renamed_chatters, cheaters, verified_players))
