# made with chatgpt
# adds the fish someone caught in the current month to their last months count so i dont have to check every url since december 2022
# doesnt consider "glimmering", "was in its mouth" and "something jumped out of the water to snatch your rare candy" as catches right now

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

# Define a dictionary to store the count of fish caught by each player
fish_catch_count = defaultdict(int)

# Define a regex pattern to extract information about fish catches
pattern = r"\s?(\w+): [@👥]\s?(\w+), You caught a [✨🫧] (.*?) [✨🫧]!"

async def fetch_data(url, renamed_chatters, cheaters, verified_players):
    async with aiohttp.ClientSession() as session:
        async with session.get(url) as response:
            text_content = await response.text()
            # Extract information about fish catches from the text content
            for match in re.finditer(pattern, text_content):
                player = match.group(2)
                
                # Check if the player is in the ignore list
                if player in cheaters:
                    continue
                
                # Change to the latest name
                player = renamed_chatters.get(player, player)
                while player in renamed_chatters:
                    player = renamed_chatters[player]
                
                # Update the fish catch count for the player
                fish_catch_count[player] += 1

async def main(renamed_chatters, cheaters, verified_players):
    for url in urls:
        await fetch_data(url, renamed_chatters, cheaters, verified_players)
    
    # Open and read the existing leaderboard file
    old_rankings = {}
    last_week_count = {}
    try:
        with open('leaderboardtotalcount.md', 'r', encoding='utf-8') as file:
            next(file) # Skip first 4 lines
            next(file)
            next(file)
            next(file)
            for line in file:
                if line.startswith("|"):
                    # Split the line and extract player, rank and their last number of fish caught
                    parts = line.split("|")
                    rank = parts[1].strip()
                    rank = rank.split()[0]
                    player = parts[2].strip()
                    count = int(parts[3].strip().split()[0])
                    if '*' in player:
                        player = player.rstrip('*')
                    
                    # Change to the latest name
                    player = renamed_chatters.get(player, player)
                    while player in renamed_chatters:
                        player = renamed_chatters[player]
                    
                    old_rankings[player] = int(rank)
                    last_week_count[player] = count
    except FileNotFoundError:
        # If the file doesn't exist, initialize old_rankings as an empty dictionary
        old_rankings = {}
    
    # get the players last months count from another leaderboard
    last_month_count = {}
    old_bot = {}
    try:
        with open('leaderboardtotalcountold.md', 'r', encoding='utf-8') as file:
                next(file) # Skip first 4 lines
                next(file)
                next(file)
                next(file)
                for line in file:
                    if line.startswith("|"):
                        parts = line.split("|")
                        count = parts[3].strip()
                        player = parts[2].strip()
                        if '*' in player:
                            player = player.rstrip('*')
                            # Change to the latest name
                            player = renamed_chatters.get(player, player)
                            while player in renamed_chatters:
                                player = renamed_chatters[player]
                            
                            last_month_count[player] = int(count)
                            old_bot[player] = "supibot"
                            
                        else:
                            # Change to the latest name
                            player = renamed_chatters.get(player, player)
                            while player in renamed_chatters:
                                player = renamed_chatters[player]
                            
                            last_month_count[player] = int(count)
                            old_bot[player] = "notsupi"
    except FileNotFoundError:
        # If the file doesn't exist, initialize as an empty dictionary
        last_month_count = {}
        old_bot = {}
                
    global fish_catch_count

    # Filter players who caught more than 100 fish
    fish_catch_count = {player: count + last_month_count.get(player, 0) for player, count in fish_catch_count.items() if count + last_month_count.get(player, 0) >= 100}

    # Add players who fished in the past to fish_catch_count
    for player in last_month_count:
        if player not in fish_catch_count and last_month_count[player] > 100:
            fish_catch_count[player] = last_month_count[player]

    # Rank the players based on the number of fish they caught
    ranked_players = sorted(fish_catch_count.items(), key=lambda x: x[1], reverse=True)

    # Assign ranks to players
    ranks = defaultdict(list)
    rank = 1
    for player, count in ranked_players:
        ranks[count].append(player)

    # Write the results to a Markdown file
    with open('leaderboardtotalcount.md', 'w', encoding='utf-8') as file:
        file.write("### Leaderboard for the most fish caught in chat (since December 2022)\n\n")
        file.write("| Rank | Player | Fish Caught |\n")
        file.write("|------|--------|-------------|\n")
        for count, players in ranks.items():
            for player in players:
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
                
                # Show the weekly change of fish caught behind the player's new count
                if player in last_week_count:
                    fish_difference = fish_catch_count[player] - last_week_count[player]
                    new_count = f"{fish_catch_count[player]} (+{fish_difference})" if fish_difference >= 1 else str(fish_catch_count[player])
                else:
                    # Player wasn't on the leaderboard last week
                    new_count = fish_catch_count[player]

                if old_bot[player] == "supibot":
                    file.write(f"| {rank} {movement[player]}| {player}* | {new_count} |\n")
                else:
                    file.write(f"| {rank} {movement[player]}| {player} | {new_count} |\n")
            rank += len(players)
        file.write("\n_* = The player caught their first fish on supibot and did not migrate their data to gofishgame. Because of that their data was not individually verified to be accurate._\n")

if __name__ == "__main__":
    renamed_chatters = renamed('lists/renamed.csv')
    cheaters = read_cheaters('lists/cheaters.txt')
    verified_players = read_verified_players('lists/verified.txt')
    asyncio.run(main(renamed_chatters, cheaters, verified_players))
