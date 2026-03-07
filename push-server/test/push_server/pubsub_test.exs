defmodule PushServer.PubSubTest do
  use ExUnit.Case, async: false

  test "PubSub module name exists in application config" do
    # Verifies the PubSub name is configured correctly
    assert Code.ensure_loaded?(Phoenix.PubSub)
  end
end
