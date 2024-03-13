# originally made with chatgpt
# updates the two leaderboards for the weekly tournaments (trophies and most fish per week)

import re
from collections import defaultdict
import csv
from itertools import groupby

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

renamed_chatters = renamed('lists/renamed.csv')
cheaters = read_cheaters('lists/cheaters.txt')
verified_players = read_verified_players('lists/verified.txt')

# Define a dictionary to store the maximum fish caught in a week for each player and the bot name
max_fish_in_week = defaultdict(lambda: {'fish_count': 0, 'bot': None})

# Define a dictionary to store the counts for each player's trophies and medals
player_counts = defaultdict(lambda: {'Trophy': 0, 'Silver': 0, 'Bronze': 0})

# Define point values
point_values = {'Trophy': 3, 'Silver': 1, 'Bronze': 0.5}

# Open and read the logs file
with open('logs/logs.txt', 'r', encoding='utf-8') as file:
    # Read each line in the file
    for line in file:

        # Find the player
        player_match = re.search(r'[@👥]\s?(\w+)', line)
        if player_match:
            player = player_match.group(1)
            
            # Skip processing for ignored players
            if player in cheaters:
                continue
            
            # Change to the latest name
            player = renamed_chatters.get(player, player)
            while player in renamed_chatters:
                player = renamed_chatters[player]

            # Get the amount of fish the player caught
            fish_match = re.search(r'(\d+) fish: (\w+)', line)
            if fish_match:
                fish_count = int(fish_match.group(1))
                bot_match = re.search(r'#\w+ \s?(\w+):', line)
                if bot_match:
                    bot = bot_match.group(1)
            
                # Update the record if the current fish count is greater
                if fish_count > max_fish_in_week[player]['fish_count']:
                    max_fish_in_week[player] = {'fish_count': fish_count, 'bot': bot}

            # Find all their medals and trophies
            achievements = re.findall(r'(Victory|champion|runner-up|third)', line)
            
            for achievement in achievements:
                if 'Victory' in achievement:
                    player_counts[player]['Trophy'] += 1
                elif 'runner-up' in achievement:
                    player_counts[player]['Silver'] += 1
                elif 'third' in achievement:
                    player_counts[player]['Bronze'] += 1
                elif 'champion' in achievement:
                    player_counts[player]['Trophy'] += 1

# Open and read the existing leaderboard files to get the old ranks
old_fish_rankings = {}
old_trophy_rankings = {}

with open('leaderboardfish.md', 'r', encoding='utf-8') as file:
    next(file) # Skip first 4 lines
    next(file)
    next(file)
    next(file)
    for line in file:
        if line.startswith("|"):
            parts = line.split("|")
            rank = parts[1].strip().split()[0]
            player = parts[2].strip()
            if '*' in player:
                player = player.rstrip('*')
            player = renamed_chatters.get(player, player)
            while player in renamed_chatters:
                player = renamed_chatters[player]
            old_fish_rankings[player] = int(rank)

with open('leaderboardtrophies.md', 'r', encoding='utf-8') as file:
    next(file) # Skip first 4 lines
    next(file)
    next(file)
    next(file)
    for line in file:
        if line.startswith("|"):
            parts = line.split("|")
            rank = parts[1].strip().split()[0]
            player = parts[2].strip()
            player = renamed_chatters.get(player, player)
            while player in renamed_chatters:
                player = renamed_chatters[player]
            old_trophy_rankings[player] = int(rank)

# Sort players by maximum fish caught and assign ranks with ties handled
rank = 0
prev_max_fish = float('inf')  # Initialize with infinity

# Write the results into a Markdown table for fish leaderboard
with open('leaderboardfish.md', 'w', encoding='utf-8') as file:
    file.write("### Leaderboard for the most fish caught in a single week in tournaments\n\n")
    file.write("| Rank | Player | Fish Caught 🪣 |\n")
    file.write("|------|--------|---------------|\n")
    for player, info in sorted(max_fish_in_week.items(), key=lambda x: x[1]['fish_count'], reverse=True):
        max_fish = info['fish_count']
        bot = info['bot']
        if max_fish >= 20:
            if max_fish < prev_max_fish:
                rank += 1
            # Ranking change
            movement = {}
            old_rank = old_fish_rankings.get(player)
            if old_rank:
                if rank < old_rank:
                    movement[player] = '⬆'
                elif rank > old_rank:
                    movement[player] = '⬇'
                else:
                    movement[player] = ''
            else:
                movement[player] = '🆕'
            if player not in verified_players and bot == 'supibot':
                file.write(f"| {rank} {movement[player]}| {player}* | {max_fish} |\n")
            else:
                file.write(f"| {rank} {movement[player]}| {player} | {max_fish} |\n")
            prev_max_fish = max_fish
    file.write("\n_* = The fish were caught on supibot and the player did not migrate their data over to gofishgame. Because of that their data was not individually verified to be accurate._\n")

# Calculate total points for each player
total_points = defaultdict(float)
for player, counts in player_counts.items():
    total_points[player] = counts['Trophy'] * point_values['Trophy'] + counts['Silver'] * point_values['Silver'] + counts['Bronze'] * point_values['Bronze']

# Sort players by total points and assign positions
sorted_players = sorted(total_points.items(), key=lambda x: x[1], reverse=True)

# Group players with the same points
grouped_players = [(points, list(group)) for points, group in groupby(sorted_players, key=lambda x: x[1])]

# Write the results into a Markdown table for trophies leaderboard
with open('leaderboardtrophies.md', 'w', encoding='utf-8') as file:
    file.write("### Leaderboard for the weekly tournaments\n\n")
    file.write("| Rank | Player | Trophies 🏆 | Silver Medals 🥈 | Bronze Medals 🥉 | Points |\n")
    file.write("|----------|--------|------------|-----------------|-----------------|--------|\n")
    for rank, (points, group) in enumerate(grouped_players, start=1):
        for player, _ in group:
            # Ranking change
            movement = {}
            old_rank = old_trophy_rankings.get(player)
            if old_rank:
                if rank < old_rank:
                    movement[player] = '⬆'
                elif rank > old_rank:
                    movement[player] = '⬇'
                else:
                    movement[player] = ''
            else:
                movement[player] = '🆕'
            file.write(f"| {rank} {movement[player]}| {player} | {player_counts[player]['Trophy']} | {player_counts[player]['Silver']} | {player_counts[player]['Bronze']} | {points} |\n")
