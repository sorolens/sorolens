/**
 * Slash-command schema definitions, kept in a separate module from the
 * runtime handlers so `register-commands.ts` can register them without
 * loading the full bot runtime (Discord client, OAuth routes, GitHub
 * client, config). That keeps the register script runnable with only
 * `DISCORD_BOT_TOKEN`, `DISCORD_CLIENT_ID`, and `DISCORD_GUILD_ID` set.
 */

import { ChannelType, SlashCommandBuilder } from "discord.js";

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
        .addChannelTypes(ChannelType.GuildText)
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
      "(Admin) Stop counting an issue toward someone's max (does not unassign on GitHub)"
    )
    .setDefaultMemberPermissions("0")
    .addIntegerOption((o) =>
      o
        .setName("issue")
        .setDescription("Issue number")
        .setMinValue(1)
        .setRequired(true)
    ),
].map((c) => c.toJSON());
