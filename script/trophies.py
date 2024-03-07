# made with chatgpt
# finds every players trophies and medals and ranks them

import re
from collections import defaultdict
from itertools import groupby
import csv

# Define point values
point_values = {'Trophy': 3, 'Silver': 1, 'Bronze': .5}

# Define a dictionary to store the counts for each player
player_counts = defaultdict(lambda: {'Trophy': 0, 'Silver': 0, 'Bronze': 0})

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

renamed_chatters = renamed('lists/renamed.csv')
cheaters = read_cheaters('lists/cheaters.txt')

# Open and read the text file
with open('logs/logs.txt', 'r', encoding='utf-8') as file:
    # Read each line in the file
    for line in file:
        # Use regular expressions to extract relevant information
        player_match = re.search(r'[@👥]\s?(\w+)', line)
        if player_match:
            player = player_match.group(1)
            
            # Check if the old name has a mapping to a new name
            player = renamed_chatters.get(player, player)

            # Skip processing for ignored players
            if player in cheaters:
                continue

            # Find all occurrences of achievements in the line
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

# Calculate total points for each player
total_points = defaultdict(float)
for player, counts in player_counts.items():
    total_points[player] = counts['Trophy'] * point_values['Trophy'] + counts['Silver'] * point_values['Silver'] + counts['Bronze'] * point_values['Bronze']

# Sort players by total points and assign positions
sorted_players = sorted(total_points.items(), key=lambda x: x[1], reverse=True)

# Group players with the same points
grouped_players = [(points, list(group)) for points, group in groupby(sorted_players, key=lambda x: x[1])]

# Open and read the existing leaderboard file
old_rankings = {}

with open('leaderboardtrophies.md', 'r', encoding='utf-8') as file:
    next(file) # Skip first 4 lines
    next(file)
    next(file)
    next(file)
    for line in file:
        if line.startswith("|"):
            # Split the line and extract player and rank
            parts = line.split("|")
            rank = parts[1].strip()
            rank = rank.split()[0]
            player = parts[2].strip()
            player = renamed_chatters.get(player, player)
            old_rankings[player] = int(rank)

# Write the results into a Markdown table
with open('leaderboardtrophies.md', 'w', encoding='utf-8') as file:
    file.write("### Leaderboard for the weekly tournaments\n\n")
    file.write("| Rank | Player | Trophies 🏆 | Silver Medals 🥈 | Bronze Medals 🥉 | Points |\n")
    file.write("|----------|--------|------------|-----------------|-----------------|--------|\n")
    for rank, (points, group) in enumerate(grouped_players, start=1):
        for player, _ in group:
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
            file.write(f"| {rank} {movement[player]}| {player} | {player_counts[player]['Trophy']} | {player_counts[player]['Silver']} | {player_counts[player]['Bronze']} | {points} |\n")
