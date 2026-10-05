#!/bin/sh
CONF=/etc/config/qpkg.conf
QPKG_NAME="BuddyFlix"
QPKG_ROOT=`/sbin/getcfg $QPKG_NAME Install_Path -f ${CONF}`
PIDFILE="$QPKG_ROOT/buddyflix.pid"
LOGFILE="$QPKG_ROOT/buddyflix.log"
export QNAP_QPKG=$QPKG_NAME

start_service() {
  ENABLED=`/sbin/getcfg $QPKG_NAME Enable -u -d FALSE -f $CONF`
  if [ "$ENABLED" != "TRUE" ]; then
    echo "$QPKG_NAME is disabled."
    exit 1
  fi

  if [ -f "$PIDFILE" ]; then
    PID=`/bin/cat "$PIDFILE" 2>/dev/null`
    if [ -n "$PID" ] && /bin/kill -0 "$PID" 2>/dev/null; then
      exit 0
    fi
  fi

  /bin/mkdir -p "$QPKG_ROOT/data"
  export BUDDYFLIX_LISTEN=":8096"
  export BUDDYFLIX_DATA="$QPKG_ROOT/data"

  cd "$QPKG_ROOT" || exit 1
  ./buddyflix >>"$LOGFILE" 2>&1 &
  /bin/echo $! > "$PIDFILE"
  /bin/sleep 2

  PID=`/bin/cat "$PIDFILE" 2>/dev/null`
  [ -n "$PID" ] && /bin/kill -0 "$PID" 2>/dev/null
}

stop_service() {
  if [ -f "$PIDFILE" ]; then
    PID=`/bin/cat "$PIDFILE" 2>/dev/null`
    if [ -n "$PID" ] && /bin/kill -0 "$PID" 2>/dev/null; then
      /bin/kill "$PID" 2>/dev/null
      i=0
      while /bin/kill -0 "$PID" 2>/dev/null && [ $i -lt 10 ]; do
        /bin/sleep 1
        i=`/usr/bin/expr $i + 1`
      done
      /bin/kill -9 "$PID" 2>/dev/null
    fi
    /bin/rm -f "$PIDFILE"
  fi
  exit 0
}

case "$1" in
  start) start_service ;;
  stop) stop_service ;;
  restart) $0 stop; $0 start ;;
  remove) stop_service ;;
  *) echo "Usage: $0 {start|stop|restart|remove}"; exit 1 ;;
esac
exit $?
