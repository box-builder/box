#!/bin/bash

sed -e "s/@@VERSION@@/$1/g" release/RELEASE.md >RELEASE.tmp.md

vim release-notes.txt
cat >CHANGELOG.tmp.md << EOF
### v${1}

EOF

cat release-notes.txt >>CHANGELOG.tmp.md
echo >>CHANGELOG.tmp.md
cat CHANGELOG.md >> CHANGELOG.tmp.md
mv CHANGELOG.tmp.md CHANGELOG.md

cat release-notes.txt >> RELEASE.tmp.md 
echo "\n\n" >> RELEASE.tmp.md

perl -i.bak -pe "s/(\\s+)Version = .*/\\1Version = \"${1}\"/" main.go
make clean all

lcuname=$(uname -s | tr LD ld)
cp $GOPATH/bin/box .
gzip -c box > "box-${1}.${lcuname}.gz"

for i in deb rpm
do
  fpm -n box -v ${1} -s dir -t ${i} box=/usr/bin/box 
done

case "$(uname -s)" in
  Darwin)
    sum=$(shasum -a 256 "box-${1}.${lcuname}.gz")
  ;;
  Linux)
    for i in "box-${1}.${lcuname}.gz" box-${1}-1.x86_64.rpm box_${1}_amd64.deb
    do
      innersum=$(sha256sum ${i})
      sum="${sum}\n${innersum}"
    done
  ;;
esac

cat >>RELEASE.tmp.md <<'EOF'
Sums:
```
EOF

echo "${sum}" >>RELEASE.tmp.md
echo '```' >>RELEASE.tmp.md

gost -p RELEASE.tmp.md CHANGELOG.md
