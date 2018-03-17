date

function test() {
  url=localhost:8080/Roomies

  for i in {0..20};
  do
    curl -Is $url | head -n1 & 2>/dev/null;
  done

  wait
}

already_running_pid=`pgrep airtabler`

if [ -z $already_running_pid ]; then
  ./airtabler --timeout 7s 2> test.log &
  airtabler_pid=$!
  echo "started airtabler - pid: $airtabler_pid"
else
  echo "using existing airtabler process $already_running_pid"
fi

time (test)

if [ -z $already_running_pid ]; then
  printf "stopping airtabler"
  kill $airtabler_pid
fi
