/**
 * Slash-command schema definitions, kept in a separate module from the
 * runtime handlers so `register-commands.ts` can register them without
 * loading the full bot runtime (Discord client, OAuth routes, GitHub
 * client, config). That keeps the register script runnable with only
 * `DISCORD_BOT_TOKEN`, `DISCORD_CLIENT_ID`, and `DISCORD_GUILD_ID` set.
 */

import { ChannelType, PermissionFlagsBits, SlashCommandBuilder } from "discord.js";

export const commandDefinitions = [
  new SlashCommandBuilder()
    .setName("connect")
    .setDescription(
      "Auto-link your GitHub via one-click OAuth (no typing needed)"
    ),
  new SlashCommandBuilder()
    .setName("link")
    .setDescription(
      "Link your GitHub account manually (advanced; most users want /connect)"
    )
    .addStringOption((o) =>
      o
        .setName("github")
        .setDescription("Your GitHub username (case-insensitive)")
        .setRequired(true)
        .setMinLength(1)
        .setMaxLength(39)
    ),
  new SlashCommandBuilder()
    .setName("unlink")
    .setDescription("Remove the GitHub link on your Discord account"),
  new SlashCommandBuilder()
    .setName("admin-unlink")
    .setDescription("(Admin) Remove another member's GitHub link")
    .setDefaultMemberPermissions("0")
    .addUserOption((o) =>
      o
        .setName("user")
        .setDescription("The Discord member to unlink")
        .setRequired(false)
    )
    .addStringOption((o) =>
      o
        .setName("github")
        .setDescription("Or the GitHub username to unlink")
        .setRequired(false)
        .setMaxLength(39)
    ),
  new SlashCommandBuilder()
    .setName("whoami")
    .setDescription("Show your current linked GitHub account, if any"),
  new SlashCommandBuilder()
    .setName("mypr")
    .setDescription(
      "Show your merged PR count and current tier for sorolens/sorolens"
    ),
  new SlashCommandBuilder()
    .setName("contributions")
    .setDescription("(Admin) Show a person's merged PR count and tier")
    .setDefaultMemberPermissions("0")
    .addUserOption((o) =>
      o
        .setName("user")
        .setDescription("A Discord member with a linked GitHub account")
        .setRequired(false)
    )
    .addStringOption((o) =>
      o
        .setName("github")
        .setDescription("A GitHub username (use instead of user)")
        .setRequired(false)
    ),
  new SlashCommandBuilder()
    .setName("members")
    .setDescription("(Admin) List linked members and their roles")
    .setDefaultMemberPermissions("0")
    .addStringOption((o) =>
      o
        .setName("filter")
        .setDescription("Filter by tier")
        .setRequired(false)
        .addChoices(
          { name: "All linked", value: "all" },
          { name: "Contributor", value: "contributor" },
          { name: "Core Contributor", value: "core" },
          { name: "No role yet", value: "none" }
        )
    ),
  new SlashCommandBuilder()
    .setName("status")
    .setDescription("Show the current Sorolens status of a tracked contract")
    .addStringOption((o) =>
      o
        .setName("contract")
        .setDescription("Stellar contract id (56 characters, starts with C)")
        .setRequired(true)
        .setMinLength(1)
        .setMaxLength(64)
        .setAutocomplete(true)
    ),
  new SlashCommandBuilder()
    .setName("alerts")
    .setDescription("Show the most recent watchdog alerts for a contract")
    .addStringOption((o) =>
      o
        .setName("contract")
        .setDescription("Stellar contract id (56 characters, starts with C)")
        .setRequired(true)
        .setMinLength(1)
        .setMaxLength(64)
        .setAutocomplete(true)
    )
    .addIntegerOption((o) =>
      o
        .setName("limit")
        .setDescription("How many alerts to show (1-10, default 5)")
        .setMinValue(1)
        .setMaxValue(10)
        .setRequired(false)
    ),
  new SlashCommandBuilder()
    .setName("watch")
    .setDescription("Add a contract to your Sorolens watchlist")
    .addStringOption((o) =>
      o
        .setName("contract")
        .setDescription("Stellar contract id (56 characters, starts with C)")
        .setRequired(true)
        .setMinLength(1)
        .setMaxLength(64)
        .setAutocomplete(true)
    ),
  new SlashCommandBuilder()
    .setName("unwatch")
    .setDescription("Remove a contract from your Sorolens watchlist")
    .addStringOption((o) =>
      o
        .setName("contract")
        .setDescription("Stellar contract id (56 characters, starts with C)")
        .setRequired(true)
        .setMinLength(1)
        .setMaxLength(64)
        .setAutocomplete(true)
    ),
  new SlashCommandBuilder()
    .setName("assign-setup")
    .setDescription(
      "(Admin) Configure issue self-assign: channel, roles, max per person"
    )
    .setDefaultMemberPermissions("0")
    .addChannelOption((o) =>
      o
        .setName("channel")
        .setDescription("Channel where members drop #issue numbers")
        .addChannelTypes(ChannelType.GuildText, ChannelType.GuildAnnouncement)
        .setRequired(false)
    )
    .addIntegerOption((o) =>
      o
        .setName("max")
        .setDescription("Max issues the bot gives one person at a time")
        .setMinValue(1)
        .setMaxValue(50)
        .setRequired(false)
    )
    .addRoleOption((o) =>
      o
        .setName("required_role")
        .setDescription("Role every applicant must have (e.g. @real)")
        .setRequired(false)
    )
    .addRoleOption((o) =>
      o
        .setName("any_role_1")
        .setDescription("Applicant also needs one of the any_role roles")
        .setRequired(false)
    )
    .addRoleOption((o) =>
      o
        .setName("any_role_2")
        .setDescription("Second accepted role (optional)")
        .setRequired(false)
    )
    .addRoleOption((o) =>
      o
        .setName("any_role_3")
        .setDescription("Third accepted role (optional)")
        .setRequired(false)
    )
    .addBooleanOption((o) =>
      o
        .setName("clear_roles")
        .setDescription("Remove all role rules before applying the ones given")
        .setRequired(false)
    ),
  new SlashCommandBuilder()
    .setName("assign-issues")
    .setDescription("(Admin) Choose which issues members can pick")
    .setDefaultMemberPermissions("0")
    .addSubcommand((s) =>
      s
        .setName("add")
        .setDescription("Open issues for self-assign")
        .addStringOption((o) =>
          o
            .setName("issues")
            .setDescription("Numbers or ranges, e.g. 836-850, 812")
            .setRequired(true)
            .setMaxLength(500)
        )
    )
    .addSubcommand((s) =>
      s
        .setName("remove")
        .setDescription("Close issues for self-assign")
        .addStringOption((o) =>
          o
            .setName("issues")
            .setDescription("Numbers or ranges, e.g. 836-850, 812")
            .setRequired(true)
            .setMaxLength(500)
        )
    )
    .addSubcommand((s) =>
      s.setName("clear").setDescription("Remove every issue from the list")
    )
    .addSubcommand((s) =>
      s.setName("list").setDescription("Show the issues members can pick")
    ),
  new SlashCommandBuilder()
    .setName("assign-toggle")
    .setDescription("(Admin) Switch issue applications on or off")
    .setDefaultMemberPermissions("0")
    .addStringOption((o) =>
      o
        .setName("state")
        .setDescription("on or off")
        .setRequired(true)
        .addChoices({ name: "on", value: "on" }, { name: "off", value: "off" })
    ),
  new SlashCommandBuilder()
    .setName("assign-status")
    .setDescription("(Admin) Show issue self-assign settings and who has what")
    .setDefaultMemberPermissions("0"),
  new SlashCommandBuilder()
    .setName("assign-release")
    .setDescription(
      "(Admin) Free issues the bot gave out (does not unassign on GitHub)"
    )
    .setDefaultMemberPermissions("0")
    .addStringOption((o) =>
      o
        .setName("issues")
        .setDescription("Issue numbers, e.g. 567, 647, 767 or 800-810")
        .setRequired(true)
    ),
  new SlashCommandBuilder()
    .setName("assign-available")
    .setDescription(
      "(Admin) List open, unassigned issues and which are in your list"
    )
    .setDefaultMemberPermissions("0")
    .addIntegerOption((o) =>
      o.setName("from").setDescription("Lowest issue number").setMinValue(1).setRequired(false)
    )
    .addIntegerOption((o) =>
      o.setName("to").setDescription("Highest issue number").setMinValue(1).setRequired(false)
    )
    .addStringOption((o) =>
      o
        .setName("label")
        .setDescription("Only issues with this label, e.g. Stellar Wave")
        .setMaxLength(100)
        .setRequired(false)
    )
    .addBooleanOption((o) =>
      o
        .setName("applicants")
        .setDescription("Also count Drips Wave applicants (slower, up to 60 issues)")
        .setRequired(false)
    ),
  new SlashCommandBuilder()
    .setName("leaderboard")
    .setDescription("Top contributors by merged PRs in sorolens/sorolens")
    .addStringOption((o) =>
      o
        .setName("period")
        .setDescription("Time range (default: all time)")
        .setRequired(false)
        .addChoices(
          { name: "This week", value: "week" },
          { name: "Last 30 days", value: "month" },
          { name: "All time", value: "all" }
        )
    ),
  new SlashCommandBuilder()
    .setName("community-setup")
    .setDescription("(Admin) Channels and switches for merge congrats and contributor of the week")
    .setDefaultMemberPermissions("0")
    .addChannelOption((o) =>
      o
        .setName("congrats_channel")
        .setDescription("Where merge congratulations go (e.g. #live-updates)")
        .addChannelTypes(ChannelType.GuildText, ChannelType.GuildAnnouncement)
        .setRequired(false)
    )
    .addChannelOption((o) =>
      o
        .setName("weekly_channel")
        .setDescription("Where contributor of the week is posted (e.g. #activity)")
        .addChannelTypes(ChannelType.GuildText, ChannelType.GuildAnnouncement)
        .setRequired(false)
    )
    .addBooleanOption((o) =>
      o.setName("congrats").setDescription("Post merge congratulations").setRequired(false)
    )
    .addBooleanOption((o) =>
      o
        .setName("contributor_of_week")
        .setDescription("Post contributor of the week on Sundays 08:00 UTC")
        .setRequired(false)
    )
    .addRoleOption((o) =>
      o
        .setName("cotw_role")
        .setDescription("Role that rotates to each week's winner")
        .setRequired(false)
    )
    .addBooleanOption((o) =>
      o.setName("clear_cotw_role").setDescription("Stop using a rotating role").setRequired(false)
    ),
  new SlashCommandBuilder()
    .setName("milestones")
    .setDescription("(Admin) Roles granted at merged-PR milestones")
    .setDefaultMemberPermissions("0")
    .addSubcommand((s) =>
      s
        .setName("add")
        .setDescription("Grant a role at a merged-PR count")
        .addIntegerOption((o) =>
          o.setName("count").setDescription("Merged PRs, e.g. 5").setMinValue(1).setRequired(true)
        )
        .addRoleOption((o) => o.setName("role").setDescription("Role to grant").setRequired(true))
    )
    .addSubcommand((s) =>
      s
        .setName("remove")
        .setDescription("Remove a milestone")
        .addIntegerOption((o) =>
          o.setName("count").setDescription("Merged PRs").setMinValue(1).setRequired(true)
        )
    )
    .addSubcommand((s) => s.setName("list").setDescription("Show milestones")),
  new SlashCommandBuilder()
    .setName("subscribe")
    .setDescription("Subscribe a channel to contract watchdog alerts")
    .setDefaultMemberPermissions(PermissionFlagsBits.ManageChannels.toString())
    .addStringOption((o) =>
      o
        .setName("contract")
        .setDescription("Stellar contract id (56 characters, starts with C)")
        .setRequired(true)
        .setMinLength(1)
        .setMaxLength(64)
        .setAutocomplete(true)
    )
    .addStringOption((o) =>
      o
        .setName("severity")
        .setDescription("Minimum alert severity to receive (default: all)")
        .setRequired(false)
        .addChoices(
          { name: "All (Info, Warning, Critical)", value: "all" },
          { name: "Critical only", value: "critical" },
          { name: "Warning and Critical", value: "warning" },
          { name: "Info only", value: "info" }
        )
    )
    .addChannelOption((o) =>
      o
        .setName("channel")
        .setDescription("Channel to subscribe (defaults to this channel)")
        .addChannelTypes(ChannelType.GuildText, ChannelType.GuildAnnouncement)
        .setRequired(false)
    ),
  new SlashCommandBuilder()
    .setName("unsubscribe")
    .setDescription("Unsubscribe a channel from contract watchdog alerts")
    .setDefaultMemberPermissions(PermissionFlagsBits.ManageChannels.toString())
    .addStringOption((o) =>
      o
        .setName("contract")
        .setDescription("Stellar contract id (56 characters, starts with C)")
        .setRequired(true)
        .setMinLength(1)
        .setMaxLength(64)
        .setAutocomplete(true)
    )
    .addChannelOption((o) =>
      o
        .setName("channel")
        .setDescription("Channel to unsubscribe (defaults to this channel)")
        .addChannelTypes(ChannelType.GuildText, ChannelType.GuildAnnouncement)
        .setRequired(false)
    ),
  new SlashCommandBuilder()
    .setName("subscriptions")
    .setDescription("List active contract alert subscriptions for a channel")
    .addChannelOption((o) =>
      o
        .setName("channel")
        .setDescription("Channel to inspect (defaults to this channel)")
        .addChannelTypes(ChannelType.GuildText, ChannelType.GuildAnnouncement)
        .setRequired(false)
    ),
].map((c) => c.toJSON());
