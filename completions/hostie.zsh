#compdef hostie
# hostie zsh completion script
# Install: copy to a directory in your $fpath (e.g., /usr/local/share/zsh/site-functions/_hostie)

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

  case $state in
    command)
      _describe 'hostie command' commands
      ;;
    args)
      case $words[1] in
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

          case $state in
            group_command)
              _describe 'group command' group_commands
              ;;
            group_args)
              case $words[1] in
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
  hostnames=(${(f)"$(hostie list --json 2>/dev/null | grep -o '"hostname":"[^"]*"' | cut -d'"' -f4)"})
  _describe 'hostname' hostnames
}

# Helper function to complete group paths
_hostie_groups() {
  local -a groups
  groups=(${(f)"$(hostie list --json 2>/dev/null | grep -o '"group":"[^"]*"' | cut -d'"' -f4 | sort -u)"})
  _describe 'group' groups
}

_hostie "$@"
