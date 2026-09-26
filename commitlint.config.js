/**
 * Conventional Commits rules for every package in the workspace.
 *
 * The commit-msg hook installed by Husky (see .husky/commit-msg) runs this
 * config locally, so an invalid message is rejected before it reaches CI.
 * The rules are documented in CONTRIBUTING.md under "Commit format".
 *
 * @type {import("@commitlint/types").UserConfig}
 */
module.exports = {
  extends: ["@commitlint/config-conventional"],
  rules: {
    // CONTRIBUTING.md documents a 72-character limit for the subject line.
    "header-max-length": [2, "always", 72],
  },
};
