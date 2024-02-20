# made with chatgpt
# updates the leaderboard of the biggest fish

import re
import asyncio
import aiohttp
from collections import defaultdict

# List of URLs containing the fish catch information
urls = [
    'https://logs.joinuv.com/channel/breadworms/user/gofishgame/2024/2?',
    # Add more URLs as needed
]

# Define a dictionary to store the old names and new names mapping
name_mapping = {'laaazuli': 'lzvli', 'mochi404': 'mochi_uygqzidbjizjkbehuiw',
                'desecrated_altar': 'miiiiisho', 'monkeycena': 'ryebreadward'}

# Define a list of players to ignore (cheaters)
players_to_ignore = ['cyancaesar', 'hansworthelias']

# List of verified players
verified_players = ['breadworms', 'trident1011', 'drainedjelqer', 'dayzedinndaydreams', 'kishma9', 'qu4ttromila', 'chubbyhamster2222', 'derinturitierutz', 
                    'puzzlow', 'xz_xz', 'paras220', 'realtechnine', 'maxlewl', 'desecrated_altar', 'sussy_amonge', 'bussinongnocap', 'sicklymaidrobot', 
                    'mochi_uygqzidbjizjkbehuiw', 'crazytown_bananapants', 'booty_bread', 'julialuxel', 'ouacewi', 'leanmeister', 'mitgliederversammlung', 
                    'squirtyraccoon', 'breedworms', 'wispmode', 'psp1g', 'k3vuit', 'eebbbee', 'dx9er', 'divra__', 'chimmothi', 'collegefifar', 'd_egree', 
                    'reapex_1', 'zwockel01', 'starducc', 'felipespe' 'flunke_', 'quton', 'fauxrothko', 'thasbe', 'thelantzzz', 'tien_', 'jackwhalebreaker', 
                    'rttvname', 'cappo7117', 'revielum', 'mazza1g', 'elraimon2000', 'seryxx', 'yopego', 'pookiesnowman', 'ovrht', 'mikel1g', 'sonigtm', 
                    'lastweeknextday', 'sameone', 'combineddota', 'lukydx', 'cowsareamazing', 'huuuuurz', 'alvaniss1g', 'sl3id3r', 'tomsi1g', 'cubedude20', 
                    'satic____', 'vibinud', 'multiplegamer9', 'yuuka7', 'device1g', 'joleksu', 'jr_mime', 'expnandbanana', 'datwguy', 'totenguden', 'xkimi1337', 
                    'ocram1g', 'breaddovariety', 'restartmikel', 'brunodestar', 'niiy', 'modestserhat', 'a1ryexpl0d1ng', 'gab_ri_el_', 'leftrights', 
                    'surelynotafishingalt', 'lugesbro', 'dubyu_', 'kaspu222', 'd0nk7', 'angus_lpc', 'faslker', 'shinespikepm', 'devonoconde', 'blapman007', 
                    'lobuhtomy', 'asthmaa', 'luzianu', 'hennnnni']

# Define a dictionary to store the details of the biggest fish caught by each player
new_record = defaultdict(lambda: {'weight': 0, 'type': None, 'bot': None})

# Define a regex pattern to extract information about fish catches
pattern = r"\s?(\w+): [@👥]\s?(\w+), You caught a [✨🫧] (.*?) [✨🫧]! It weighs ([\d.]+) lbs"

async def fetch_data(url):
    async with aiohttp.ClientSession() as session:
        async with session.get(url) as response:
            text_content = await response.text()
            # Extract information about fish catches from the text content
            for match in re.finditer(pattern, text_content):
                bot, player, fish_type, fish_weight_str = match.groups()
                if player in players_to_ignore:
                    continue  # Skip processing for ignored players
                # Check if the player name has a mapping to a new name
                player = name_mapping.get(player, player)
                fish_weight = float(fish_weight_str)
                # Store the bot where the fish was caught
                if fish_weight > new_record[player]['weight']:
                    new_record[player] = {'weight': fish_weight, 'type': fish_type, 'bot': bot}

async def main():
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
                # Check if the ⚠ marker is present indicating 'supibot'
                if '⚠' in line:
                    bot = 'supibot'  # Set bot as 'supibot' if the marker is present
                old_record[player] = {'weight': fish_weight, 'type': fish_type, 'bot': bot}


    # Process logs to get new records
    for url in urls:
        await fetch_data(url)

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
                    file.write(f"#{rank} {player}: {fish_details['type']} {fish_details['weight']} lbs ⚠️\n")
                else:
                    file.write(f"#{rank} {player}: {fish_details['type']} {fish_details['weight']} lbs\n")
        file.write("⚠️ This means that the fish was caught on supibot and the player did not migrate their data over to gofishgame. Because of that their data was not individually verified to be accurate.\n")

if __name__ == "__main__":
    asyncio.run(main())
