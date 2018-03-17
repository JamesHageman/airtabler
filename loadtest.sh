function test() {
  url=localhost:8080/Roomies

  for i in {0..20};
  do
    AIRTABLER_LOADTEST=1 curl -Is $url | head -n1 & 2>/dev/null;
  done

  wait
}

date
time test
