make race

for i in {0..20};
do
  (AIRTABLER_LOADTEST=1 curl -Is localhost:8080/Roomies | head -n1 &) 2>/dev/null;
done

# TODO: this never stops
until `pgrep AIRTABLER_LOADTEST`; do
  printf '.'
  sleep 1
done
