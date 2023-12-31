# Missed Activity Notifier (MAN) Plugin

A plugin for Mattermost to notify users by email about unread messages in the Mattermost server that might be of interest for them.

Mattermost already sends email notifications, but only if the user is offline or away and only for direct messages, mentions or followed threads (see [here](https://docs.mattermost.com/about/faq-notifications.html)). It is done on purpose to avoid sending too many emails and because Mattermost notifies already all messages with desktop and mobile notifications, so emails are the last resort for very important messages.

However, in small teams, with a small messages rate and users that are often offline, important messages could be missed because never notified by Mattermost or due to missed desktop notifications. This plugin tries to fill this gap sending email notifications for messages for which Mattermost would never send an email.

The plugin works by periodically collecting all unread messages for users, excluding messages that should have been already notified by Mattermost or that are in channels that the user explicitly muted, and sending notification emails for the remaining messages (aggregated by team).

This plugin does not replace the standard Mattermost email notification mechanism, it works in conjunction with it, excluding from its notifications messages that should have been already notified by Mattermost.


⚠️ **The plugin is in development state and needs further testing to be sure it can work in all situations. I tested only in one server with around 30 users and very few messages. Also, it is the first plugin for Mattermost I write and also my first project in Golang, so I'm sure the code can be greatly improved.**
## User Preferences

The plugin works autonomously after the initial configuration done at administration level (see "Plugin Configuration" section). However, each user can customize some aspects of the plugin using the `/missedactivity` [slash command](https://docs.mattermost.com/integrations/cloud-slash-commands.html).

### Show the current user's preferences

```
/missedactivity prefs show
```

### Enable/Disable the Plugin

Activate:
```
/missedactivity prefs Enabled true
```

Deactivate:
```
/missedactivity prefs Enabled false
```
### Replies in not followed threads

The plugin can be configured to send notifications also for messages that are sent in threads that the user is not following. To activate this feature use:
```
/missedactivity prefs NotifyRepliesInNotFollowedThreads true
```
To disable the feature use:
```
/missedactivity prefs NotifyRepliesInNotFollowedThreads false
```

If the feature is enabled, the counter *"replies in not followed threads"* is not shown

### Message Types

Beside messages from users, the plugin can be configured to also notify system messages (e.g. "@UserX joined the team", "@UserY left the channel.") or messages that comes from bots like the Boards or the GitLab bots.

The user can choose to be notified for these messages using:
```
/missedactivity prefs IncludeSystemMessages true
/missedactivity prefs IncludeMessagesFromBots true
```

or ignore these messages using:
```
/missedactivity prefs IncludeSystemMessages false
/missedactivity prefs IncludeMessagesFromBots false
```

### Counters

Notification emails can also include three different type of counters to inform the user about additional **unread** messages not shown in the email itself:

- *number of replies in not followed threads*
- *number of messages notified by Mattermost itself (and not by this plugin) and still unread*
- *number of messages notified by this plugin previously and still unread*

These counters can be activated using the following commands:

```
/missedactivity prefs IncludeCountOfRepliesInNotFollowedThreads true
/missedactivity prefs InlcudeCountOfMessagesNotifiedByMM true
/missedactivity prefs IncludeCountPreviouslyNotified true
```

or disabled using:
```
/missedactivity prefs IncludeCountOfRepliesInNotFollowedThreads false
/missedactivity prefs InlcudeCountOfMessagesNotifiedByMM false
/missedactivity prefs IncludeCountPreviouslyNotified false
```

## Q&A

### How do I stop receiving emails only from a specific channel?
You can mute the channel in the channel configuration (this will stop also the standard Mattermost email notifications). Otehrwise you can decide to leave a channel if you are not interested at all in the channel.

### How do I stop receiving emails only from this plugin?

You can disable the plugin issuing the command `/missedactivity prefs Enabled false` command.

### How do I stop receiving all emails from Mattermost?
You can disable email notifications in the *Settings -> Email Notifications* section. This will stop not only emails from this plugin, but all emails send by Mattermost.

### Why did I received two notifications for the same message?

Due to limitations in the Mattermost plugin API, this plugin cannot directly know if the Mattermost server already sent a notification for a given message. The plugin tries to simulate the Mattermost logic to understand if an email notification for the message could have been already sent or not. However, this mechanism is not 100% accurate and in some cases (especially for unread messages created before the plugin was started) it might result in a message notified twice.

### Why did I not receive any notification for a given message?
See answer to the previous FAQ.

### Why did I receive multiple notification emails at the same time?

The plugin aggregate unread messaged by team and sends one distinct email for each team you are member. This helps to make it clear to what team the messages you are reading in the email notification belongs to. Direct messages does not belong to any specific team and they are notified all together in a distinct email. The only exception to this rule is if you are member of just one team. In this case, you will receive a single notification email that includes both messages from the team and all the direct messages.

## Admin Configuration

The plugin has several configuration parameters. For most of them, the default value is good in most of the use cases. In fact, they have been introduced during the development more for debugging purposes than real customization needs.

⚠️ **After the first installation, please, set "Dry Run" to "false" to start sending emails.** By default, the "Dry Run" parameter is true, for saftey reasons, otherwise the plugin will start to send emails as soon as it has been installed without the possibility for the administartor to configure it.

| Key                                    | Description                                                                                                                                                                                                                                                                                                                                                                                                             | Default Value                                                                                                                                                                                                                           |
|----------------------------------------|-------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|-----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| `DryRun`                                 | Do not send emails, just log the execution. Useful for debugging and testing purposes                                                                                                                                                                                                                                                                                                                                   | true                                                                                                                                                                                                                                    |
| `RunInterval`                            | The time interval **in minutes** with which the plugin will check for unread messages and will send email notifications. *This interval also influences the internal caches expiration time (set at interval/2)*                                                                                                                                                                                                        | 180 (3 hours)                                                                                                                                                                                                                           |
| `IgnoreMessagesNewerThan`               | The minimum time **in minutes** before notifyng a new message. When the plugin runs (determined by *Run Interval*), messages newer than this time period will be ignored (they will be processed in the next run).                                                                                                                                                                                                      | 30                                                                                                                                                                                                                                      |
| `NotifyOnlyNewMessagesFromStartup`       | If true only messages posted after the plugin startup time will be considered by the plugin. If false, on the first run the plugin will process all messages from the last notified timestamp (stored in the database). This affect not only the messages, that will appear in the emails, but also the counters.                                                                                                       | false                                                                                                                                                                                                                                   |
| `KeepStatusHistoryInterval`              | The plugin records and keeps in memory the status of users to calculate if Mattermost already sent some email notifications and avoid sending it again. This interval (expressed in **minutes**) specifies for how long data will be kept. This should be at least equal to *RunInterval*. Keeping it for an interval longer than that increments the accuracy of the counters that appears in the notification emails. | 168 (one week)                                                                                                                                                                                                                          |
| `UserDefaultPrefEnabled`                 | If true, the plugin is active for all users by default and needs to be explicitly disabled on per-user basis. If false, the plugin is disabled unless the user explicitly activate                                                                                                                                                                                                                                      | true                                                                                                                                                                                                                                    |
| `UserDefaultPrefNotifyNotFollowed`       | Whether to include or not in notification emails the unread replies in not followed threads. This is the default value and can be overridden on per-user basis                                                                                                                                                                                                                                                          | false                                                                                                                                                                                                                                   |
| `UserDefaultIncludeSystemMessages`                 | Whether to include or not in notification emails the unread system messages (e.g., users join/leaving a channel). This is the default value and can be overridden on per-user                                                                                                                                                                                                                                       | true                                                                                                                                                                                                                                    |
| `UserDefaultPrefIncludeMessagesFromBots` | Whether to include or not in notification emails the unread messages from bots. This is the default value and can be overridden on per-user basis                                                                                                                                                                                                                                                                       | true                                                                                                                                                                                                                                    |
| `UserDefaultPrefCountNotFollowed`        | Whether to include or not in notification emails the count of unread replies in not followed threads (useful if UserDefaultPrefNotifyNotFollowed is false). This is the default value and can be overridden on per-user basis                                                                                                                                                                                           | true                                                                                                                                                                                                                                    |
| `UserDefaultPrefCountMM`                 | Whether to include or not in notification emails the count of unread messages already notified by Mattermost. This is the default value and can be overridden on per-user basis                                                                                                                                                                                                                                         | true                                                                                                                                                                                                                                    |
| `UserDefaultPrefCountPreviouslyNotified` | Whether to include or not in notification emails the count of messages notified in previous emails, but still unread. This is the default value and can be overridden on per-user basis                                                                                                                                                                                                                                 | true                                                                                                                                                                                                                                    |
| `EmailSubTitle`                          | The message that will appear in the notification above the list of messages                                                                                                                                                                                                                                                                                                                                             | Since the last time you connected, new messages have been posted that might be of interest for you                                                                                                                                      |
| `EmailButtonText`                        | The text of the message in the button that will open the Mattermost website                                                                                                                                                                                                                                                                                                                                             | See in Mattermost                                                                                                                                                                                                                       |
| `EmailFooterLine1`                       | The text of the first line of the footer that will appear in the emails                                                                                                                                                                                                                                                                                                                                                 | You are receiving this email from the Missed Activity Plugin. Use the command \"/missedactivity help\" in Mattermost to know more and configure the behaviour of the plugin.                                                            |
| `EmailFooterLine2`                       | The text of the second line of the footer that will appear in the emails                                                                                                                                                                                                                                                                                                                                                |                                                                                                                                                                                                                                         |
| `EmailFooterLine3`                       | The text of the third line of the footer that will appear in the emails                                                                                                                                                                                                                                                                                                                                                 | This email is sent from the Missed Activity Notifier plugin. Use the \"/missedactivity help\" command in Mattermost to know more. If you think you should have not received this message, please contact your Mattermost administrator. |
| `DebugLogEnabled`                        | If true print all message logs, otherwise print only error, warning and info level messages                                                                                                                                                                                                                                                                                                                             | false                                                                                                                                                                                                                                   |
| `DebugHTTPToken`                         | A token to protect the http endpoint to access debug logs                                                                                                                                                                                                                                                                                                                                                               | null                                                                                                                                                                                                                                    |
| `RunStatsToKeep`                         | For each run, the plugin keeps in memeory logs and outputs for debugging and explaination purposes. While the size of this data is very tiny, after a a given number of runs, they are deleted to free memory                                                                                                                                                                                                           | false                                                                                                                                                                                                                                   |
| `ResetLastNotificationTimestamp`         | Resets the last notified timestamp at startup. This is the timestamp that MAN stores at each run that indicate from what point in time the next run should start to process unread messages                                                                                                                                                                                                                             | false                                                                                                                                                                                                                                   |
