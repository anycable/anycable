require 'active_model'

class User
  include ActiveModel::Model
  include GlobalID::Identification

  attr_accessor :name, :secret

  def self.find_by_gid(gid)
    new(gid.params)
  end

  # For gid
  def id
    0
  end

  def to_global_id(_options)
    super(
      app: :anycable,
      name: name,
      secret: secret
    )
  end

  GlobalID::Locator.use :anycable do |gid|
    gid.model_name.constantize.find_by_gid(gid)
  end
end
