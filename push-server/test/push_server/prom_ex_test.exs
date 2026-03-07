defmodule PushServer.PromExTest do
  use ExUnit.Case, async: true

  test "PromEx module exists" do
    assert Code.ensure_loaded?(PushServer.PromEx)
  end

  test "plugins/0 returns a list" do
    plugins = PushServer.PromEx.plugins()
    assert is_list(plugins)
    assert length(plugins) > 0
  end

  test "plugins includes PromEx.Plugins.Beam" do
    plugins = PushServer.PromEx.plugins()
    beam_plugin = Enum.find(plugins, fn
      PromEx.Plugins.Beam -> true
      _ -> false
    end)
    assert beam_plugin != nil
  end

  test "CustomPlugin module exists" do
    assert Code.ensure_loaded?(PushServer.PromEx.CustomPlugin)
  end
end
