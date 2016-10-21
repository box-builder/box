MRuby::Build.new do |conf|
  # load specific toolchain settings

  # Gets set by the VS command prompts.
  if ENV['VisualStudioVersion'] || ENV['VSINSTALLDIR']
    toolchain :visualcpp
  else
    toolchain :gcc
  end

  if ENV["MRUBY_DEBUG"]
    enable_debug
  end

  gems = %W[
    mruby-sprintf
    mruby-print
    mruby-math
    mruby-time
    mruby-struct
    mruby-enum-ext
    mruby-string-ext
    mruby-numeric-ext
    mruby-array-ext
    mruby-hash-ext
    mruby-range-ext
    mruby-proc-ext
    mruby-symbol-ext
    mruby-random
    mruby-object-ext
    mruby-enumerator
    mruby-enum-lazy
  ]

  gems.each { |gem| conf.gem File.join(root, "mrbgems", gem) }
end
