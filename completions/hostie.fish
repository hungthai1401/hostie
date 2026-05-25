# hostie fish completion script
# Install: copy to ~/.config/fish/completions/hostie.fish

# Remove old completions
complete -c hostie -e

# Main commands
complete -c hostie -f -n "__fish_use_subcommand" -a "add" -d "Add a new host entry"
complete -c hostie -f -n "__fish_use_subcommand" -a "rm" -d "Remove a host entry"
complete -c hostie -f -n "__fish_use_subcommand" -a "enable" -d "Enable a host entry"
complete -c hostie -f -n "__fish_use_subcommand" -a "disable" -d "Disable a host entry"
complete -c hostie -f -n "__fish_use_subcommand" -a "edit" -d "Edit a host entry"
complete -c hostie -f -n "__fish_use_subcommand" -a "list" -d "List all host entries"
complete -c hostie -f -n "__fish_use_subcommand" -a "apply" -d "Apply changes to /etc/hosts"
complete -c hostie -f -n "__fish_use_subcommand" -a "group" -d "Manage groups"
complete -c hostie -f -n "__fish_use_subcommand" -a "completion" -d "Generate shell completion script"
complete -c hostie -f -n "__fish_use_subcommand" -a "version" -d "Show version information"
complete -c hostie -f -n "__fish_use_subcommand" -a "help" -d "Show help information"

# add command options
complete -c hostie -f -n "__fish_seen_subcommand_from add" -l group -d "Group path" -a "(hostie list --json 2>/dev/null | string match -r '\"group\":\"[^\"]*\"' | string replace -r '.*\"group\":\"([^\"]*)\".*' '\$1' | sort -u)"
complete -c hostie -f -n "__fish_seen_subcommand_from add" -l alias -d "Alias name"
complete -c hostie -f -n "__fish_seen_subcommand_from add" -l disabled -d "Create disabled"
complete -c hostie -f -n "__fish_seen_subcommand_from add" -l comment -d "Comment"

# rm, enable, disable, edit - complete with hostnames
complete -c hostie -f -n "__fish_seen_subcommand_from rm" -a "(hostie list --json 2>/dev/null | string match -r '\"hostname\":\"[^\"]*\"' | string replace -r '.*\"hostname\":\"([^\"]*)\".*' '\$1')"
complete -c hostie -f -n "__fish_seen_subcommand_from enable" -a "(hostie list --json 2>/dev/null | string match -r '\"hostname\":\"[^\"]*\"' | string replace -r '.*\"hostname\":\"([^\"]*)\".*' '\$1')"
complete -c hostie -f -n "__fish_seen_subcommand_from disable" -a "(hostie list --json 2>/dev/null | string match -r '\"hostname\":\"[^\"]*\"' | string replace -r '.*\"hostname\":\"([^\"]*)\".*' '\$1')"
complete -c hostie -f -n "__fish_seen_subcommand_from edit" -a "(hostie list --json 2>/dev/null | string match -r '\"hostname\":\"[^\"]*\"' | string replace -r '.*\"hostname\":\"([^\"]*)\".*' '\$1')"

# list command options
complete -c hostie -f -n "__fish_seen_subcommand_from list" -l group -d "Filter by group" -a "(hostie list --json 2>/dev/null | string match -r '\"group\":\"[^\"]*\"' | string replace -r '.*\"group\":\"([^\"]*)\".*' '\$1' | sort -u)"
complete -c hostie -f -n "__fish_seen_subcommand_from list" -l json -d "Output as JSON"

# apply command options
complete -c hostie -f -n "__fish_seen_subcommand_from apply" -l dry-run -d "Show changes without applying"

# group subcommands
complete -c hostie -f -n "__fish_seen_subcommand_from group; and not __fish_seen_subcommand_from add rm list mv" -a "add" -d "Create a new group"
complete -c hostie -f -n "__fish_seen_subcommand_from group; and not __fish_seen_subcommand_from add rm list mv" -a "rm" -d "Remove a group"
complete -c hostie -f -n "__fish_seen_subcommand_from group; and not __fish_seen_subcommand_from add rm list mv" -a "list" -d "List all groups"
complete -c hostie -f -n "__fish_seen_subcommand_from group; and not __fish_seen_subcommand_from add rm list mv" -a "mv" -d "Move/rename a group"

# group rm options
complete -c hostie -f -n "__fish_seen_subcommand_from group; and __fish_seen_subcommand_from rm" -l force -d "Force removal of non-empty group"
complete -c hostie -f -n "__fish_seen_subcommand_from group; and __fish_seen_subcommand_from rm" -a "(hostie list --json 2>/dev/null | string match -r '\"group\":\"[^\"]*\"' | string replace -r '.*\"group\":\"([^\"]*)\".*' '\$1' | sort -u)"

# group mv - complete with groups
complete -c hostie -f -n "__fish_seen_subcommand_from group; and __fish_seen_subcommand_from mv" -a "(hostie list --json 2>/dev/null | string match -r '\"group\":\"[^\"]*\"' | string replace -r '.*\"group\":\"([^\"]*)\".*' '\$1' | sort -u)"

# completion command - complete with shell names
complete -c hostie -f -n "__fish_seen_subcommand_from completion" -a "bash zsh fish"

# help command - complete with command names
complete -c hostie -f -n "__fish_seen_subcommand_from help" -a "add rm enable disable edit list apply group completion version help"
