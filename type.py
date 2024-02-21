# made with chatgpt
# finds the biggest fish per fish type and who caught it and updates the leaderboard
# jellyfish is a bttv emote currently

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

# Define a mapping for equivalent fish types
equivalent_fish_types = {
    '🕷': '🕷️', '🗡' : '🗡️', '🕶' : '🕶️', '☂' : '☂️', '⛸' : '⛸️', '🧜♀' : '🧜‍♀️', '🧜♀️' : '🧜‍♀️', '🧜‍♀' : '🧜‍♀️', '🐻‍❄️' : '🐻‍❄', '🧞‍♂️' : '🧞‍♂', 'HailHelix' : '🐚',
}

not_fish_types = {'🛢️', '🦂', 'HailHelix'}

# Define a list of players to ignore (cheaters)
players_to_ignore = ['cyancaesar', 'hansworthelias']

# Define a dictionary to store the biggest fish of each fish type
biggest_fish_by_type = defaultdict(lambda: {'player': '', 'weight': 0})

# Define a regex pattern to extract information about fish catches
pattern = r"\s?(\w+): [@👥]\s?(\w+), You caught a [✨🫧] (.*?) [✨🫧]! It weighs ([\d.]+) lbs"

async def fetch_data(url):
    async with aiohttp.ClientSession() as session:
        async with session.get(url) as response:
            text_content = await response.text()
            # Extract information about fish catches from the text content
            for match in re.finditer(pattern, text_content):
                bot, player, fish_type, fish_weight_str = match.groups()
                fish_weight = float(fish_weight_str)
                # Check if the player name has a mapping to a new name
                player = name_mapping.get(player, player)
                if player in players_to_ignore:
                    continue  # Skip processing for ignored players
                if fish_type in not_fish_types:
                    continue  
                if fish_type in equivalent_fish_types:
                    fish_type = equivalent_fish_types[fish_type]  # Update fish type if it has an equivalent
                # Update the biggest fish of each fish type
                if fish_weight > biggest_fish_by_type[fish_type]['weight']:
                    biggest_fish_by_type[fish_type] = {'player': player, 'weight': fish_weight, 'bot': bot}

async def main():
    for url in urls:
        await fetch_data(url)

    # Read existing leaderboard data from leaderboardtype.txt
    existing_leaderboard = {}
    with open('leaderboardtype.txt', 'r', encoding='utf-8') as file:
        leaderboard_content = file.readlines()[1:]
        for line in leaderboard_content:
            if line.strip():  # Skip empty lines
                # Use regex to extract fish type, weight, and player
                match = re.match(r"(\S+)\s+([\d.]+)\s+lbs,\s+([\w*]+)", line.strip())
                if match:
                    fish_type, weight_str, player = match.groups()
                    weight = float(weight_str)
                    bot = None
                    if '*' in player:
                        player = player.rstrip('*')
                        bot = 'supibot'
                    existing_leaderboard[fish_type] = {'weight': weight, 'player': player, 'bot': bot}

    # Compare fetched data with existing data and update the leaderboard if necessary
    updated_leaderboard = {}
    # Copy existing leaderboard to the updated leaderboard
    for fish_type, fish_info in existing_leaderboard.items():
        updated_leaderboard[fish_type] = fish_info

    # Update the updated leaderboard with new data
    for fish_type, fish_info in biggest_fish_by_type.items():
        if fish_type in existing_leaderboard:
            if fish_info['weight'] > existing_leaderboard[fish_type]['weight']:
                updated_leaderboard[fish_type] = fish_info
        else:
            updated_leaderboard[fish_type] = fish_info

    # Sort fish types based on their weights
    sorted_leaderboard = sorted(updated_leaderboard.items(), key=lambda x: x[1]['weight'], reverse=True)

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
    asyncio.run(main())
