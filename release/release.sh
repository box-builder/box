#!/bin/bash

sed -e "s/@@VERSION@@/$1/g" release/RELEASE.md >RELEASE.tmp.md
sed -e "s/@@VERSION@@/$1/g" install.sh.tmpl >install.sh

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


gzip -c box > "box-${1}.linux.gz"

sed -e "s/@@VERSION@@/${1}/g" portable.sh.tmpl >portable.sh
gzip -c portable.sh > "box-${1}.portable.gz"

rm *.deb *.rpm

fpm -n box -v ${1} -s dir -t deb box=/usr/bin/box
fpm -n box -v ${1} -s dir -t rpm box=/usr/bin/box

case "$(uname -s)" in
  Linux)
    for i in "box-${1}.portable.gz" "box-${1}.linux.gz" "box-${1}-1.x86_64.rpm" "box_${1}_amd64.deb"
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
