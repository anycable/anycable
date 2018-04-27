# mruby build configurations

MRuby::Build.new do |conf|
  # load specific toolchain settings

  # Gets set by the VS command prompts.
  if ENV['VisualStudioVersion'] || ENV['VSINSTALLDIR']
    toolchain :visualcpp
  else
    toolchain :gcc
  end

  # enable_debug

  # Use mrbgems
  # conf.gem 'examples/mrbgems/ruby_extension_example'
  # conf.gem 'examples/mrbgems/c_extension_example' do |g|
  #   g.cc.flags << '-g' # append cflags in this gem
  # end
  # conf.gem 'examples/mrbgems/c_and_ruby_extension_example'
  # conf.gem :github => 'masuidrive/mrbgems-example', :checksum_hash => '76518e8aecd131d047378448ac8055fa29d974a9'
  # conf.gem :git => 'git@github.com:masuidrive/mrbgems-example.git', :branch => 'master', :options => '-v'

  # include the default GEMs
  conf.gembox 'default'
  # C compiler settings
  # conf.cc do |cc|
  #   cc.command = ENV['CC'] || 'gcc'
  #   cc.flags = [ENV['CFLAGS'] || %w()]
  #   cc.include_paths = ["#{root}/include"]
  #   cc.defines = %w(DISABLE_GEMS)
  #   cc.option_include_path = '-I%s'
  #   cc.option_define = '-D%s'
  #   cc.compile_options = "%{flags} -MMD -o %{outfile} -c %{infile}"
  # end

  # mrbc settings
  # conf.mrbc do |mrbc|
  #   mrbc.compile_options = "-g -B%{funcname} -o-" # The -g option is required for line numbers
  # end

  # Linker settings
  # conf.linker do |linker|
  #   linker.command = ENV['LD'] || 'gcc'
  #   linker.flags = [ENV['LDFLAGS'] || []]
  #   linker.flags_before_libraries = []
  #   linker.libraries = %w()
  #   linker.flags_after_libraries = []
  #   linker.library_paths = []
  #   linker.option_library = '-l%s'
  #   linker.option_library_path = '-L%s'
  #   linker.link_options = "%{flags} -o %{outfile} %{objs} %{libs}"
  # end

  # Archiver settings
  # conf.archiver do |archiver|
  #   archiver.command = ENV['AR'] || 'ar'
  #   archiver.archive_options = 'rs %{outfile} %{objs}'
  # end

  # Parser generator settings
  # conf.yacc do |yacc|
  #   yacc.command = ENV['YACC'] || 'bison'
  #   yacc.compile_options = '-o %{outfile} %{infile}'
  # end

  # gperf settings
  # conf.gperf do |gperf|
  #   gperf.command = 'gperf'
  #   gperf.compile_options = '-L ANSI-C -C -p -j1 -i 1 -g -o -t -N mrb_reserved_word -k"1,3,$" %{infile} > %{outfile}'
  # end

  # file extensions
  # conf.exts do |exts|
  #   exts.object = '.o'
  #   exts.executable = '' # '.exe' if Windows
  #   exts.library = '.a'
  # end

  # file separetor
  # conf.file_separator = '/'

  # bintest
  # conf.enable_bintest
end

if ENV['MRUBY_CROSS_OS'] == 'linux'
  MRuby::CrossBuild.new("linux-arm64") do |conf|
    toolchain :gcc

    url = 'http://crossgcc.rts-software.org/doku.php?id=compiling_for_linux'

    cgcc = "/usr/local/gcc-4.8.1-for-linux64/bin/x86_64-pc-linux-gcc"
    car = "/usr/local/gcc-4.8.1-for-linux64/bin/x86_64-pc-linux-ar"

    fail "Can't find #{cgcc}. Please download compiler from #{url}" unless File.exist? cgcc
    fail "Can't find #{car}. Please download compiler from #{url}" unless File.exist? car

    MRuby.targets["host"].gems.each do |mrbgem|
      conf.gem mrbgem.dir
    end

    conf.cc.command = cgcc
    conf.cc.flags << "-static"
    conf.linker.command = cgcc
    conf.archiver.command = car
  end

  MRuby::CrossBuild.new("linux-amd64") do |conf|
    toolchain :gcc

    url = 'http://crossgcc.rts-software.org/doku.php?id=compiling_for_linux'

    cgcc = "/usr/local/gcc-4.8.1-for-linux64/bin/x86_64-pc-linux-gcc"
    car = "/usr/local/gcc-4.8.1-for-linux64/bin/x86_64-pc-linux-ar"

    fail "Can't find #{cgcc}. Please download compiler from #{url}" unless File.exist? cgcc
    fail "Can't find #{car}. Please download compiler from #{url}" unless File.exist? car

    MRuby.targets["host"].gems.each do |mrbgem|
      conf.gem mrbgem.dir
    end

    conf.cc.command = cgcc
    conf.cc.flags <<  %w(-fPIC -static)
    conf.linker.command = cgcc
    conf.archiver.command = car
  end

  MRuby::CrossBuild.new("linux-386") do |conf|
    toolchain :gcc

    url = 'http://crossgcc.rts-software.org/doku.php?id=compiling_for_linux'

    cgcc = "/usr/local/gcc-4.8.1-for-linux32/bin/i586-pc-linux-gcc"
    car = "/usr/local/gcc-4.8.1-for-linux32/bin/i586-pc-linux-ar"

    fail "Can't find #{cgcc}. Please download compiler from #{url}" unless File.exist? cgcc
    fail "Can't find #{car}. Please download compiler from #{url}" unless File.exist? car

    MRuby.targets["host"].gems.each do |mrbgem|
      conf.gem mrbgem.dir
    end

    conf.cc.command = cgcc
    conf.cc.flags << "-static"
    conf.linker.command = cgcc
    conf.archiver.command = car
  end
end
