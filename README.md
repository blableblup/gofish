## Unofficial leaderboards for gofish 🥇
* gofish made by [breadworms](https://www.twitch.tv/breadworms)

* [gofish.lol](https://gofish.lol/) website with the leaderboards

## About the boards

* The leaderboards and the profiles are remade every sunday with data up to saturday. This way, the changes match the tournament week. The tournament leaderboards are updated later because players need to do +checkin in chat to see their result.

* The * next to a player's name on the leaderboards indicates that they did not migrate their supibot data to gofishgame. Thus, their supibot records are marked with an asterisk because they were not verified for accuracy.

* If there are multiple records with the same weight for the "Biggest / Smallest fish per type" leaderboards, only the player who caught it the earliest is displayed. 

* The "Smallest fish per type" and "Averageweight" leaderboards do not include fish you get from releasing and squirrels, because they do not show their weight in the catch message. (There were three squirrels which had their weight all set to be the same, they will show up there)

## About the data

* The program parses the logs of gofishgame (or supibot for older data) and then inserts the fish and the tournament results into a postgresql database. Fish from Twitch whispers, from Discord and fish caught before the justlog instance was added to the chat are not included. (To see which chats are being covered look here: [config](https://github.com/blableblup/gofish/blob/main/config.json))

* The logs are probably not fully complete in most cases, but they should contain the vast majority of fish. If a chat has mutliple justlog instances, I can check the different ones and find missing fish. The only instance with a lot of very long downtimes was logs nadeko, but that was only used for psp1g and those gaps have been fixed.

* Some gaps I know about which weren't filled: Small gap for [logs.spanix.team](https://logs.spanix.team/channel/omie/user/gofishgame/2025/1) from 2025-01-21 05:18:08 to 2025-01-23 01:32:54 for omie. And somewhere in this week 2025-02-09/16 for vaiastol.

* Fish seen through gifts 🎁 and through releasing to another player during the winter events were not added to the database.