## Unofficial leaderboards for gofish 🥇
gofish made by [breadworms](https://www.twitch.tv/breadworms)

[gofish.juliapixel](https://gofish.lol/) website with the leaderboards

_The leaderboards are updated every sunday_

To see which chats are being covered look here: [config](https://github.com/blableblup/gofish/blob/main/config.json)

## About the data 🗃️

* The program currently only works for Twitch chats. Fish caught before the justlog instance was added to the chat are not included, nor are fish caught in Twitch whispers or on Discord.

* The program parses the logs of gofishgame (or supibot for older data) and then inserts the fish and the tournament results into a postgresql database.

* The logs are probably not fully complete in most cases, but they should contain the vast majority of fish. 

* The log data for psp1g's chat from the 27th of February 2024 to the 3rd of March 2024 is incomplete (see [here](https://logs.nadeko.net/channel/psp1g/2024/2/28)).

* The data for psp1g's chat from the 12th of December 2023 to the 14th of December 2023  is also incomplete (see [here](https://logs.nadeko.net/channel/psp1g/2023/12/13)).

* The * next to a player's name on the leaderboard indicates that they did not migrate their supibot data to gofishgame. Thus, their supibot records are marked with an asterisk because they were not verified for accuracy.