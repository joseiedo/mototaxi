defmodule PushServer.RedixBehaviour do
  @moduledoc "Behaviour for Redix commands, allowing Mox injection in tests."
  @callback command(atom(), list()) :: {:ok, any()} | {:error, any()}
end
