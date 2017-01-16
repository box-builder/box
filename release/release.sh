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

cp $GOPATH/bin/box .

lcuname=$(uname -s | tr LD ld)

perl -i.bak -e "s/(\\s+)Version = .*/\\1Version = \"${1}\"/" main.go

make

gzip -c box > "box-${1}.${lcuname}.gz"

case "$(uname -s)" in
  Darwin)
    sum=$(shasum -a 256 "box-${1}.${lcuname}.gz")
  ;;
  Linux)
    sum=$(sha256sum "box-${1}.${lcuname}.gz")
  ;;
esac

cat >>RELEASE.tmp.md <<'EOF'
Sums:
```
EOF

echo "${sum}" >>RELEASE.tmp.md
echo '```' >>RELEASE.tmp.md

gost -p RELEASE.tmp.md CHANGELOG.md
