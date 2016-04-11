SCRIPT=$(basename $0)
mkdir -p /var/vcap/sys/log/monit

exec 1>> /var/vcap/sys/log/monit/$SCRIPT.log
exec 2>> /var/vcap/sys/log/monit/$SCRIPT.err.log

echo "------------ `basename $0` $* at `date` --------------" | tee /dev/stderr

function pid_is_running() {
  declare pid="$1"
  ps -p "${pid}" >/dev/null 2>&1
}

# pid_guard
#
# @param pidfile
# @param name [String] an arbitrary name that might show up in STDOUT on errors
#
# Run this before attempting to start new processes that may use the same :pidfile:.
# If an old process is running on the pid found in the :pidfile:, exit 1. Otherwise,
# remove the stale :pidfile: if it exists.
#
function pid_guard() {
  declare pidfile="$1" name="$2"

  echo "------------ STARTING $(basename "$0") at $(date) --------------" | tee /dev/stderr

  if [ ! -f "${pidfile}" ]; then
    return 0
  fi

  local pid
  pid=$(head -1 "${pidfile}")

  if pid_is_running "${pid}"; then
    echo "${name} is already running, please stop it first"
    exit 1
  fi

  echo "Removing stale pidfile"
  rm "${pidfile}"
}

# wait_pid_death
#
# @param pid
# @param timeout
#
# Watch a :pid: for :timeout: seconds, waiting for it to die.
# If it dies before :timeout:, exit 0. If not, exit 1.
#
# Note that this should be run in a subshell, so that the current
# shell does not exit.
#
function wait_pid_death() {
  declare pid="$1" timeout="$2"

  local countdown
  countdown=$(( timeout * 10 ))

  while true; do
    if ! pid_is_running "${pid}"; then
      return 0
    fi

    if [ ${countdown} -le 0 ]; then
      return 1
    fi

    countdown=$(( countdown - 1 ))
    sleep 0.1
  done
}

# kill_and_wait
#
# @param pidfile
# @param timeout [default 25s]
#
# For a pid found in :pidfile:, send a `kill -6`, then wait for :timeout: seconds to
# see if it dies on its own. If not, send it a `kill -9`. If the process does die,
# exit 0 and remove the :pidfile:. If after all of this, the process does not actually
# die, exit 1.
#
# Note:
# Monit default timeout for start/stop is 30s
# Append 'with timeout {n} seconds' to monit start/stop program configs
#
function kill_and_wait() {
  declare pidfile="$1" timeout="${2:-25}" sigkill_on_timeout="${3:-1}"

  if [ ! -f "${pidfile}" ]; then
    echo "Pidfile ${pidfile} doesn't exist"
    exit 0
  fi

  local pid
  pid=$(head -1 "${pidfile}")

  if [ -z "${pid}" ]; then
    echo "Unable to get pid from ${pidfile}"
    exit 1
  fi

  if ! pid_is_running "${pid}"; then
    echo "Process ${pid} is not running"
    rm -f "${pidfile}"
    exit 0
  fi

  echo "Killing ${pidfile}: ${pid} "
  kill "${pid}"

  if ! wait_pid_death "${pid}" "${timeout}"; then
    if [ "${sigkill_on_timeout}" = "1" ]; then
      echo "Kill timed out, using kill -9 on ${pid}"
      kill -9 "${pid}"
      sleep 0.5
    fi
  fi

  if pid_is_running "${pid}"; then
    echo "Timed Out"
    exit 1
  else
    echo "Stopped"
    rm -f "${pidfile}"
  fi
}

check_mount() {
  opts=$1
  exports=$2
  mount_point=$3

  if grep -qs $mount_point /proc/mounts; then
    echo "Found NFS mount $mount_point"
  else
    echo "Mounting NFS..."
    mount $opts $exports $mount_point
    if [ $? != 0 ]; then
      echo "Cannot mount NFS from $exports to $mount_point, exiting..."
      exit 1
    fi
  fi
}

# Check the syntax of a sudoers file.
check_sudoers() {
  /usr/sbin/visudo -c -f "$1"
}

# Check the syntax of a sudoers file and if it's ok install it.
install_sudoers() {
  src="$1"
  dest="$2"

  check_sudoers "$src"

  if [ $? -eq 0 ]; then
    chown root:root "$src"
    chmod 0440 "$src"
    cp -p "$src" "$dest"
  else
    echo "Syntax error in sudoers file $src"
    exit 1
  fi
}

# Add a line to a file if it is not already there.
file_must_include() {
  file="$1"
  line="$2"

  # Protect against empty $file so it doesn't wait for input on stdin.
  if [ -n "$file" ]; then
    grep --quiet "$line" "$file" || echo "$line" >> "$file"
  else
    echo 'File name is required'
    exit 1
  fi
}

running_in_container() {
  grep -q -E '/instance|/docker/' /proc/self/cgroup
}
