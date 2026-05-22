/**
 * Completion command implementation
 * 
 * Generates shell completion scripts for bash, zsh, and fish
 */

export type Shell = "bash" | "zsh" | "fish";

/**
 * Execute the completion command
 * 
 * @param shell - Target shell (bash, zsh, or fish)
 * @returns Exit code (0 = success, 1 = unsupported shell)
 */
export async function completionCommand(shell: Shell): Promise<number> {
  switch (shell) {
    case "bash":
      console.log(BASH_COMPLETION);
      return 0;
    case "zsh":
      console.log(ZSH_COMPLETION);
      return 0;
    case "fish":
      console.log(FISH_COMPLETION);
      return 0;
    default:
      console.error(`Error: Unsupported shell "${shell}". Supported shells: bash, zsh, fish`);
      return 1;
  }
}

// Bash completion script
const BASH_COMPLETION = `# hostie bash completion script
# Install: source this file or copy to /etc/bash_completion.d/hostie

_hostie_completion() {
  local cur prev words cword
  _init_completion || return

  local commands="add rm enable disable edit list apply group completion version help"
  local group_commands="add rm list mv"

  # Complete main commands
  if [[ \${cword} -eq 1 ]]; then
    COMPREPLY=( \$(compgen -W "\${commands}" -- "\${cur}") )
    return 0
  fi

  # Get the main command
  local cmd="\${words[1]}"

  case "\${cmd}" in
    add)
      case "\${prev}" in
        --group)
          # Complete with group paths from hostie list
          local groups=\$(hostie list --json 2>/dev/null | grep -o '"group":"[^"]*"' | cut -d'"' -f4 | sort -u)
          COMPREPLY=( \$(compgen -W "\${groups}" -- "\${cur}") )
          ;;
        *)
          if [[ \${cur} == -* ]]; then
            COMPREPLY=( \$(compgen -W "--group --alias --disabled --comment" -- "\${cur}") )
          fi
          ;;
      esac
      ;;
    rm|enable|disable|edit)
      # Complete with hostnames from hostie list
      if [[ \${cword} -eq 2 ]]; then
        local hostnames=\$(hostie list --json 2>/dev/null | grep -o '"hostname":"[^"]*"' | cut -d'"' -f4)
        COMPREPLY=( \$(compgen -W "\${hostnames}" -- "\${cur}") )
      fi
      ;;
    list)
      case "\${prev}" in
        --group)
          local groups=\$(hostie list --json 2>/dev/null | grep -o '"group":"[^"]*"' | cut -d'"' -f4 | sort -u)
          COMPREPLY=( \$(compgen -W "\${groups}" -- "\${cur}") )
          ;;
        *)
          if [[ \${cur} == -* ]]; then
            COMPREPLY=( \$(compgen -W "--group --json" -- "\${cur}") )
          fi
          ;;
      esac
      ;;
    apply)
      if [[ \${cur} == -* ]]; then
        COMPREPLY=( \$(compgen -W "--dry-run" -- "\${cur}") )
      fi
      ;;
    group)
      if [[ \${cword} -eq 2 ]]; then
        COMPREPLY=( \$(compgen -W "\${group_commands}" -- "\${cur}") )
      elif [[ \${cword} -eq 3 ]]; then
        case "\${words[2]}" in
          rm|mv)
            local groups=\$(hostie list --json 2>/dev/null | grep -o '"group":"[^"]*"' | cut -d'"' -f4 | sort -u)
            COMPREPLY=( \$(compgen -W "\${groups}" -- "\${cur}") )
            ;;
        esac
      fi
      ;;
    completion)
      if [[ \${cword} -eq 2 ]]; then
        COMPREPLY=( \$(compgen -W "bash zsh fish" -- "\${cur}") )
      fi
      ;;
    help)
      if [[ \${cword} -eq 2 ]]; then
        COMPREPLY=( \$(compgen -W "\${commands}" -- "\${cur}") )
      fi
      ;;
  esac
}

complete -F _hostie_completion hostie
`;

// Zsh completion script
const ZSH_COMPLETION = `#compdef hostie
# hostie zsh completion script
# Install: copy to a directory in your \$fpath (e.g., /usr/local/share/zsh/site-functions/_hostie)

_hostie() {
  local -a commands
  commands=(
    'add:Add a new host entry'
    'rm:Remove a host entry'
    'enable:Enable a host entry'
    'disable:Disable a host entry'
    'edit:Edit a host entry'
    'list:List all host entries'
    'apply:Apply changes to /etc/hosts'
    'group:Manage groups'
    'completion:Generate shell completion script'
    'version:Show version information'
    'help:Show help information'
  )

  local -a group_commands
  group_commands=(
    'add:Create a new group'
    'rm:Remove a group'
    'list:List all groups'
    'mv:Move/rename a group'
  )

  _arguments -C \
    '1: :->command' \
    '*:: :->args'

  case \$state in
    command)
      _describe 'hostie command' commands
      ;;
    args)
      case \$words[1] in
        add)
          _arguments \
            '1:ip address:' \
            '2:hostname:' \
            '*:aliases:' \
            '--group[Group path]:group:_hostie_groups' \
            '--alias[Alias name]:alias:' \
            '--disabled[Create disabled]' \
            '--comment[Comment]:comment:'
          ;;
        rm|enable|disable|edit)
          _arguments '1:hostname:_hostie_hostnames'
          ;;
        list)
          _arguments \
            '--group[Filter by group]:group:_hostie_groups' \
            '--json[Output as JSON]'
          ;;
        apply)
          _arguments '--dry-run[Show changes without applying]'
          ;;
        group)
          _arguments \
            '1: :->group_command' \
            '*:: :->group_args'

          case \$state in
            group_command)
              _describe 'group command' group_commands
              ;;
            group_args)
              case \$words[1] in
                add)
                  _arguments '1:group path:'
                  ;;
                rm)
                  _arguments \
                    '1:group path:_hostie_groups' \
                    '--force[Force removal of non-empty group]'
                  ;;
                mv)
                  _arguments \
                    '1:source path:_hostie_groups' \
                    '2:destination path:'
                  ;;
              esac
              ;;
          esac
          ;;
        completion)
          _arguments '1:shell:(bash zsh fish)'
          ;;
        help)
          _describe 'command' commands
          ;;
      esac
      ;;
  esac
}

# Helper function to complete hostnames
_hostie_hostnames() {
  local -a hostnames
  hostnames=(\${(f)"$(hostie list --json 2>/dev/null | grep -o '"hostname":"[^"]*"' | cut -d'"' -f4)"})
  _describe 'hostname' hostnames
}

# Helper function to complete group paths
_hostie_groups() {
  local -a groups
  groups=(\${(f)"$(hostie list --json 2>/dev/null | grep -o '"group":"[^"]*"' | cut -d'"' -f4 | sort -u)"})
  _describe 'group' groups
}

_hostie "$@"
`;

// Fish completion script
const FISH_COMPLETION = `# hostie fish completion script
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
`;
