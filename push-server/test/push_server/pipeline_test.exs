defmodule PushServer.PipelineTest do
  use ExUnit.Case, async: true

  test "Pipeline module exists" do
    assert Code.ensure_loaded?(PushServer.Pipeline)
  end

  test "handle_message/3 is exported" do
    assert function_exported?(PushServer.Pipeline, :handle_message, 3)
  end

  test "handle_failed/2 is exported" do
    assert function_exported?(PushServer.Pipeline, :handle_failed, 2)
  end
end
