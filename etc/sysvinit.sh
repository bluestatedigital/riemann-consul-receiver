#!/bin/bash
#
# riemann-consul-receiver        Manage the consul-to-riemann receiver
#       
# chkconfig:   2345 95 95
# description: Feeds Consul check results to Riemann
# processname: riemann-consul-receiver
# config: /etc/sysconfig/riemann-consul-receiver
# pidfile: /var/run/riemann-consul-receiver.pid

### BEGIN INIT INFO
# Provides:       riemann-consul-receiver
# Required-Start: $local_fs $network
# Required-Stop:
# Should-Start:
# Should-Stop:
# Default-Start: 2 3 4 5
# Default-Stop:  0 1 6
# Short-Description: Manage the consul agent
# Description: Feeds Consul check results to Riemann
### END INIT INFO

# source function library
. /etc/rc.d/init.d/functions

prog="riemann-consul-receiver"
user="consul"
exec="/usr/bin/$prog"
pidfile="/var/run/$prog.pid"
lockfile="/var/lock/subsys/$prog"
logfile="/var/log/$prog"
conffile="/etc/sysconfig/$prog"

# pull in sysconfig settings
[ -e $conffile ] && . $conffile

export GOMAXPROCS=${GOMAXPROCS:-2}
export LOG_FILE="$logfile"

export RIEMANN_HOST
export RIEMANN_PORT
export RIEMANN_PROTO
export CONSUL_HOST
export CONSUL_PORT
export UPDATE_INTERVAL
export LOCK_DELAY
export DEBUG

start() {
    [ -x $exec ] || exit 5
    
    [ -f $conffile ] || exit 6

    umask 077

    touch $logfile $pidfile
    chown $user:$user $logfile $pidfile

    echo -n $"Starting $prog: "
    
    ## holy shell shenanigans, batman!
    ## go can't be properly daemonized.  we need the pid of the spawned process,
    ## which is actually done via runuser thanks to --user.
    ## you can't do "cmd &; action" but you can do "{cmd &}; action".
    ##
    ## riemann-consul-receiver will not write to stdout except in the case of a
    ## runtime panic
    daemon \
        --pidfile=$pidfile \
        --user=$user \
        " { $exec > $logfile 2>&1 & } ; echo \$! >| $pidfile "
    
    RETVAL=$?
    
    if [ $RETVAL -eq 0 ]; then
        touch $lockfile
    fi
    
    echo    
    return $RETVAL
}

stop() {
    echo -n $"Stopping $prog: "
    
    killproc -p $pidfile $prog
    RETVAL=$?

    if [ $RETVAL -eq 0 ]; then
        rm -f $lockfile $pidfile
    fi

    echo
    return $RETVAL
}

restart() {
    stop
    start
}

force_reload() {
    restart
}

rh_status() {
    status -p "$pidfile" -l $prog $exec
    
    RETVAL=$?
    
    return $RETVAL
}

rh_status_q() {
    rh_status >/dev/null 2>&1
}

case "$1" in
    start)
        rh_status_q && exit 0
        $1
        ;;
    stop)
        rh_status_q || exit 0
        $1
        ;;
    restart)
        $1
        ;;
    force-reload)
        force_reload
        ;;
    status)
        rh_status
        ;;
    condrestart|try-restart)
        rh_status_q || exit 0
        restart
        ;;
    *)
        echo $"Usage: $0 {start|stop|status|restart|condrestart|try-restart|force-reload}"
        exit 2
esac

exit $?
