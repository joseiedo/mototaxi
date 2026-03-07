defmodule PushServer.PubSubTest do
  use ExUnit.Case, async: true

  test "Phoenix.PubSub module is loadable" do
    assert Code.ensure_loaded?(Phoenix.PubSub)
  end

  test "Application module supervision tree is defined" do
    assert Code.ensure_loaded?(PushServer.Application)
    assert function_exported?(PushServer.Application, :start, 2)
  end

  test "CustomerChannel has terminate/2 exported" do
    assert Code.ensure_loaded?(PushServerWeb.CustomerChannel)
    assert function_exported?(PushServerWeb.CustomerChannel, :terminate, 2)
  end
end
