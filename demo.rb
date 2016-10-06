from "debian"
3.times do
  run "echo hi >>root-test"
end

run "mkdir /test"
run "chown nobody /test"

user "nobody" do
  workdir "/test" do
    3.times do
      run "echo hi >>test"
    end
  end
end
