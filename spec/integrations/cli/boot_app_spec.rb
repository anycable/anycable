# frozen_string_literal: true

require "fileutils"
require "spec_helper"

describe "CLI require app", :cli do
  context "when no application provided" do
    it "prints error and exit" do
      run_cli do |cli|
        expect(cli).to have_output_line("Couldn't find an application to load")
        expect(cli).to have_stopped
        expect(cli).to have_exit_status(1)
      end
    end
  end

  context "when config/environment.rb is present" do
    before do
      FileUtils.mkdir(File.join(PROJECT_ROOT, "bin/config"))
      FileUtils.cp(
        File.join(PROJECT_ROOT, "spec/dummies/rails.rb"),
        File.join(PROJECT_ROOT, "bin/config/environment.rb")
      )
    end

    after do
      FileUtils.rm_rf(File.join(PROJECT_ROOT, "bin/config"))
    end

    it "loads Rails application" do
      run_cli do |cli|
        expect(cli).to have_output_line("Serving Rails application from ./config/environment.rb")
      end
    end
  end

  context "when require option is present" do
    it "loads application when file exists" do
      run_cli("-r ../spec/dummies/app.rb") do |cli|
        expect(cli).to have_output_line("Serving application from ../spec/dummies/app.rb")
        expect(cli).to have_output_line("Hello from app, server!")
      end
    end

    it "prints application when file couldn't be loaded" do
      run_cli("-r ../spec/dummies/fake.rb") do |cli|
        expect(cli).to have_output_line("cannot load such file")
        expect(cli).to have_stopped
        expect(cli).to have_exit_status(1)
      end
    end
  end
end
