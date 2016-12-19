Dir.chdir(File.join(__dir__, "../"))

VERSION_RXP = /^\#\# Version ([\.\-\w]+)$/
PATH_RXP = /anycable\-go\-[\.\w\-]+\-(\w+)\-\w+$/

PLATFORM_NAMES = {
  'macos' => 'MacOS',
  'linux' => 'Linux',
  'freebsd' => 'FreeBSD',
  'win' => 'Windows'
}.freeze

contents = []

version = `git describe --always --tags`.chomp

contents << "\#\# Version #{version}\n"

bin_groups = Dir["dist/*"].group_by do |path|
  path.match(PATH_RXP)[1]
end

bin_groups.each do |k, files|
  contents << "\#\#\# #{PLATFORM_NAMES[k]}\n"

  files.each do |path|
    basepath = File.basename(path)
    contents << "- [#{basepath}](https://s3.amazonaws.com/anycable/builds/#{version}/#{basepath})"
  end

  contents << ""
end

old_contents = (File.exist?("DOWNLOADS.md") ? File.read("DOWNLOADS.md") : '').split("\n")

if !old_contents.empty? && (vmatch = old_contents[0].match(VERSION_RXP))
  # Overwrite downloads for the same version
  if vmatch[1] == version
    old_contents = old_contents.drop(1)
    old_contents = old_contents.drop_while { |str| str !~ VERSION_RXP }
    old_contents = old_contents.drop(1)
  end
end

contents += old_contents

File.write("DOWNLOADS.md", contents.join("\n"))
