# made with chatgpt
# counts how many fish someone caught in chat and ranks them
# doesnt consider "glimmering", "was in its mouth" and "something jumped out of the water to snatch your rare candy" as catches right now
# has to check the logs from the beginning 

import re
import asyncio
import aiohttp
from collections import defaultdict

# List of URLs containing the fish catch information
urls = [
    'https://logs.joinuv.com/channel/breadworms/user/supibot/2022/12?',
    'https://logs.joinuv.com/channel/breadworms/user/supibot/2023/1?',
    'https://logs.joinuv.com/channel/breadworms/user/supibot/2023/2?',
    'https://logs.joinuv.com/channel/breadworms/user/supibot/2023/3?',
    'https://logs.joinuv.com/channel/breadworms/user/supibot/2023/4?',
    'https://logs.joinuv.com/channel/breadworms/user/supibot/2023/5?',
    'https://logs.joinuv.com/channel/breadworms/user/supibot/2023/6?',
    'https://logs.joinuv.com/channel/breadworms/user/supibot/2023/7?',
    'https://logs.joinuv.com/channel/breadworms/user/supibot/2023/8?',
    'https://logs.joinuv.com/channel/breadworms/user/supibot/2023/9?',
    'https://logs.joinuv.com/channel/breadworms/user/gofishgame/2023/9?',
    'https://logs.joinuv.com/channel/breadworms/user/gofishgame/2023/10?',
    'https://logs.joinuv.com/channel/breadworms/user/gofishgame/2023/11?',
    'https://logs.joinuv.com/channel/breadworms/user/gofishgame/2023/12?',
    'https://logs.joinuv.com/channel/breadworms/user/gofishgame/2024/1?',
    'https://logs.joinuv.com/channel/breadworms/user/gofishgame/2024/2?',
    # Add more URLs as needed
]

# Define a dictionary to store the old names and new names mapping
name_mapping = {'laaazuli': 'lzvli', 'mochi404': 'mochi_uygqzidbjizjkbehuiw',
                'desecrated_altar': 'miiiiisho', 'monkeycena': 'ryebreadward'}

# List of verified players
verified_players = ['breadworms', 'trident1011', 'drainedjelqer', 'dayzedinndaydreams', 'kishma9', 'qu4ttromila', 'chubbyhamster2222', 'derinturitierutz', 
                    'puzzlow', 'xz_xz', 'paras220', 'realtechnine', 'maxlewl', 'desecrated_altar', 'miiiiisho', 'sussy_amonge', 'bussinongnocap', 'sicklymaidrobot', 
                    'mochi_uygqzidbjizjkbehuiw', 'crazytown_bananapants', 'booty_bread', 'julialuxel', 'ouacewi', 'leanmeister', 'mitgliederversammlung', 
                    'squirtyraccoon', 'breedworms', 'wispmode', 'psp1g', 'k3vuit', 'eebbbee', 'dx9er', 'divra__', 'chimmothi', 'collegefifar', 'd_egree', 
                    'reapex_1', 'zwockel01', 'starducc', 'felipespe' 'flunke_', 'quton', 'fauxrothko', 'thasbe', 'thelantzzz', 'tien_', 'jackwhalebreaker', 
                    'rttvname', 'cappo7117', 'revielum', 'mazza1g', 'elraimon2000', 'seryxx', 'yopego', 'pookiesnowman', 'ovrht', 'mikel1g', 'sonigtm', 
                    'lastweeknextday', 'sameone', 'combineddota', 'lukydx', 'cowsareamazing', 'huuuuurz', 'alvaniss1g', 'sl3id3r', 'tomsi1g', 'cubedude20', 
                    'satic____', 'vibinud', 'multiplegamer9', 'yuuka7', 'device1g', 'joleksu', 'jr_mime', 'expnandbanana', 'datwguy', 'totenguden', 'xkimi1337', 
                    'ocram1g', 'breaddovariety', 'restartmikel', 'brunodestar', 'niiy', 'modestserhat', 'a1ryexpl0d1ng', 'gab_ri_el_', 'leftrights', 
                    'surelynotafishingalt', 'lugesbro', 'dubyu_', 'kaspu222', 'd0nk7', 'angus_lpc', 'faslker', 'shinespikepm', 'devonoconde', 'blapman007', 
                    'lobuhtomy', 'asthmaa', 'luzianu', 'hennnnni']

# Define a dictionary to store the count of fish caught by each player
fish_catch_count = defaultdict(int)

# Define a list of players to ignore
players_to_ignore = ['cyancaesar', 'hansworthelias']

# Define a regex pattern to extract information about fish catches
pattern = r"\s?(\w+): [@👥]\s?(\w+), You caught a [✨🫧] (.*?) [✨🫧]!"

# Keep track of players who caught their first fish with supibot
first_catch_with_supibot = defaultdict(bool)

async def fetch_data(url):
    async with aiohttp.ClientSession() as session:
        async with session.get(url) as response:
            text_content = await response.text()
            # Extract information about fish catches from the text content
            for match in re.finditer(pattern, text_content):
                player_name = match.group(2)
                bot_name = match.group(1)
                # Check if the player is in the ignore list
                if player_name in players_to_ignore:
                    continue
                # Check if the player name has a mapping to a new name
                player_name = name_mapping.get(player_name, player_name)
                # Add a verification check
                if bot_name == "supibot" and player_name not in verified_players and not first_catch_with_supibot[player_name]:
                    first_catch_with_supibot[player_name] = True
                # Update the fish catch count for the player
                fish_catch_count[player_name] += 1

async def main():
    global fish_catch_count  # Declare fish_catch_count as global
    for url in urls:
        await fetch_data(url)

    # Filter players who caught more than 100 fish
    fish_catch_count = {player: count for player, count in fish_catch_count.items() if count > 100}

    # Rank the players based on the number of fish they caught
    ranked_players = sorted(fish_catch_count.items(), key=lambda x: x[1], reverse=True)

    # Assign ranks to players
    ranks = defaultdict(list)
    rank = 1
    for player, count in ranked_players:
        ranks[count].append(player)

    # Write the results to a text file
    with open('leaderboardtotalcount.txt', 'w', encoding='utf-8') as file:
        file.write("The most fish caught in chat (since December 2022):\n")
        for count, players in ranks.items():
            for player in players:
                if first_catch_with_supibot[player] == True:
                    file.write(f"#{rank} {player}*: {count} fish caught \n")
                else:
                    file.write(f"#{rank} {player}: {count} fish caught \n")
            rank += len(players)
        file.write("* = The player caught their first fish on supibot and did not migrate their data to gofishgame. Because of that their data was not individually verified to be accurate.\n")

if __name__ == "__main__":
    asyncio.run(main())
