# made with chatgpt
# gets each players individual record for most fish in a week and ranks them

import re
from collections import defaultdict
import csv

# Define a dictionary to store the maximum fish caught in a week for each player and the bot name
max_fish_in_week = defaultdict(lambda: {'fish_count': 0, 'bot_name': None})

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

renamed_chatters = renamed('lists/renamed.csv')
cheaters = read_cheaters('lists/cheaters.csv')
verified_players = read_verified_players('lists/verified.csv')

# Open and read the text file
with open('logs/logs.txt', 'r', encoding='utf-8') as file:
    # Read each line in the file
    for line in file:
        # Use regular expressions to extract relevant information
        fish_match = re.search(r'🪣 (\d+) fish: (\w+)', line)
        if fish_match:
            fish_count = int(fish_match.group(1))
            bot_match = re.search(r'#breadworms \s?(\w+):', line)
            if bot_match:
                bot_name = bot_match.group(1)

                # Extract the player name from the line
                player_match = re.search(r'[@👥]\s?(\w+),', line)
                if player_match:
                    player = next(filter(None, player_match.groups()))  # Filter out None values
                    # Check if the player is in the ignore list
                    if player in cheaters:
                        continue  # Skip processing for ignored players

                    # Check if the player name has a mapping to a new name
                    new_player = renamed_chatters.get(player, player)
                    # Update the record if the current fish count is greater
                    if fish_count > max_fish_in_week[new_player]['fish_count']:
                        max_fish_in_week[new_player] = {'fish_count': fish_count, 'bot_name': bot_name}

# Sort players by maximum fish caught and assign ranks with ties handled
rank = 0
prev_max_fish = float('inf')  # Initialize with infinity

# Write the results into a text file
with open('leaderboardfish.txt', 'w', encoding='utf-8') as file:
    file.write("Chatters and their most fish caught in a single week in tournaments:\n")
    for player, info in sorted(max_fish_in_week.items(), key=lambda x: x[1]['fish_count'], reverse=True):
        max_fish = info['fish_count']
        bot_name = info['bot_name']
        # Check if the player meets the minimum threshold
        if max_fish >= 20:
            if max_fish < prev_max_fish:
                rank += 1
            if player not in verified_players and bot_name == 'supibot':
                file.write(f"#{rank} {player}*: {max_fish} fish\n")
            else:
                file.write(f"#{rank} {player}: {max_fish} fish\n")
            prev_max_fish = max_fish
    file.write("* = The fish were caught on supibot and the player did not migrate their data over to gofishgame. Because of that their data was not individually verified to be accurate.\n")
