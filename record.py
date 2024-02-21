# made with chatgpt
# gets each players individual record for most fish in a week and ranks them

import re
from collections import defaultdict

# Define a dictionary to store the maximum fish caught in a week for each player and the bot name
max_fish_in_week = defaultdict(lambda: {'fish_count': 0, 'bot_name': None})

# Define a dictionary to store the old names and new names mapping
name_mapping = {'laaazuli': 'lzvli', 'mochi404': 'mochi_uygqzidbjizjkbehuiw', 
                'desecrated_altar': 'miiiiisho', 'monkeycena': 'ryebreadward'}

# Define a list of players to ignore
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

# Open and read the text file
with open('logs.txt', 'r', encoding='utf-8') as file:
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
                    if player in players_to_ignore:
                        continue  # Skip processing for ignored players

                    # Check if the player name has a mapping to a new name
                    new_player = name_mapping.get(player, player)
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
