# made with chatgpt
# gets each players individual record for most fish in a week and ranks them

import re
from collections import defaultdict

# Define a dictionary to store the maximum fish caught in a week for each player
max_fish_in_week = defaultdict(list)

# Define a dictionary to store the old names and new names mapping
name_mapping = {'laaazuli': 'lzvli', 'mochi404': 'mochi_uygqzidbjizjkbehuiw', 'desecrated_altar': 'miiiiisho', 'monkeycena': 'ryebreadward'}

# Open and read the text file
with open('logs.txt', 'r', encoding='utf-8') as file:
    # Read each line in the file
    for line in file:
        # Use regular expressions to extract relevant information
        fish_match = re.search(r'🪣 (\d+) fish:', line)
        if fish_match:
            fish_count = int(fish_match.group(1))

            # Extract the player name from the line
            player_match = re.search(r'[@👥]\s?(\w+),', line)
            if player_match:
                player = next(filter(None, player_match.groups()))  # Filter out None values
                # Check if the player name has a mapping to a new name
                new_player = name_mapping.get(player, player)
                max_fish_in_week[new_player].append(fish_count)

# Sort players by maximum fish caught and assign ranks with ties handled
rank = 0
prev_max_fish = float('inf')  # Initialize with infinity

# Write the results into a text file
with open('leaderboardfish.txt', 'w', encoding='utf-8') as result_file:
    for player, fish_counts in sorted(max_fish_in_week.items(), key=lambda x: max(x[1]), reverse=True):
        max_fish = max(fish_counts)
        if max_fish < prev_max_fish:
            rank += 1
        result_file.write(f"#{rank} {player}: {max_fish} fish\n")
        prev_max_fish = max_fish