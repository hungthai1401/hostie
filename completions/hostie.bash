# hostie bash completion script
# Install: source this file or copy to /etc/bash_completion.d/hostie

_hostie_completion() {
  local cur prev words cword
  _init_completion || return

  local commands="add rm enable disable edit list apply group completion version help"
  local group_commands="add rm list mv"

  # Complete main commands
  if [[ ${cword} -eq 1 ]]; then
    COMPREPLY=( $(compgen -W "${commands}" -- "${cur}") )
    return 0
  fi

  # Get the main command
  local cmd="${words[1]}"

  case "${cmd}" in
    add)
      case "${prev}" in
        --group)
          # Complete with group paths from hostie list
          local groups=$(hostie list --json 2>/dev/null | grep -o '"group":"[^"]*"' | cut -d'"' -f4 | sort -u)
          COMPREPLY=( $(compgen -W "${groups}" -- "${cur}") )
          ;;
        *)
          if [[ ${cur} == -* ]]; then
            COMPREPLY=( $(compgen -W "--group --alias --disabled --comment" -- "${cur}") )
          fi
          ;;
      esac
      ;;
    rm|enable|disable|edit)
      # Complete with hostnames from hostie list
      if [[ ${cword} -eq 2 ]]; then
        local hostnames=$(hostie list --json 2>/dev/null | grep -o '"hostname":"[^"]*"' | cut -d'"' -f4)
        COMPREPLY=( $(compgen -W "${hostnames}" -- "${cur}") )
      fi
      ;;
    list)
      case "${prev}" in
        --group)
          local groups=$(hostie list --json 2>/dev/null | grep -o '"group":"[^"]*"' | cut -d'"' -f4 | sort -u)
          COMPREPLY=( $(compgen -W "${groups}" -- "${cur}") )
          ;;
        *)
          if [[ ${cur} == -* ]]; then
            COMPREPLY=( $(compgen -W "--group --json" -- "${cur}") )
          fi
          ;;
      esac
      ;;
    apply)
      if [[ ${cur} == -* ]]; then
        COMPREPLY=( $(compgen -W "--dry-run" -- "${cur}") )
      fi
      ;;
    group)
      if [[ ${cword} -eq 2 ]]; then
        COMPREPLY=( $(compgen -W "${group_commands}" -- "${cur}") )
      elif [[ ${cword} -eq 3 ]]; then
        case "${words[2]}" in
          rm|mv)
            local groups=$(hostie list --json 2>/dev/null | grep -o '"group":"[^"]*"' | cut -d'"' -f4 | sort -u)
            COMPREPLY=( $(compgen -W "${groups}" -- "${cur}") )
            ;;
        esac
      fi
      ;;
    completion)
      if [[ ${cword} -eq 2 ]]; then
        COMPREPLY=( $(compgen -W "bash zsh fish" -- "${cur}") )
      fi
      ;;
    help)
      if [[ ${cword} -eq 2 ]]; then
        COMPREPLY=( $(compgen -W "${commands}" -- "${cur}") )
      fi
      ;;
  esac
}

complete -F _hostie_completion hostie
