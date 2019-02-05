# frozen_string_literal: true

namespace :load do
  task :defaults do
    set :anycable_default_hooks, true
    set :anycable_roles, :app

    set :anycable_env,                   -> { fetch(:rack_env, fetch(:rails_env, fetch(:stage))) }
    set :anycable_environment_variables, {}
    set :anycable_conf,                  nil

    # For internal use only
    set :_anycable_command, :anycabled
    set :_anycable_environment, lambda {
      fetch(:default_env).merge(fetch(:anycable_environment_variables)).merge(
        {
          rails_env: fetch(:anycable_env),
          anycable_conf: fetch(:anycable_conf)
        }.compact
      )
    }

    # Bundler integration
    set :bundle_bins, fetch(:bundle_bins).to_a.concat(%w[anycabled])
    # Rbenv, Chruby, and RVM integration
    set :rbenv_map_bins, fetch(:rbenv_map_bins).to_a.concat(%w[anycabled])
    set :rvm_map_bins, fetch(:rvm_map_bins).to_a.concat(%w[anycabled])
    set :chruby_map_bins, fetch(:chruby_map_bins).to_a.concat(%w[anycabled])
  end
end

namespace :deploy do
  before :starting, :check_anycable_hooks do
    invoke "anycable:add_default_hooks" if fetch(:anycable_default_hooks)
  end
end

namespace :anycable do
  task :add_default_hooks do
    after "deploy:restart", "anycable:restart"
  end

  desc "Start anycable process"
  task :start do
    anycabled :start
  end

  desc "Stop anycable process"
  task :stop do
    anycabled :stop
  end

  desc "Restart anycable process"
  task :restart do
    anycabled :restart
  end

  def anycabled(command)
    on roles fetch(:anycable_roles) do
      within release_path do
        with fetch(:_anycable_environment) do
          execute :_anycable_command, command
        end
      end
    end
  end
end
