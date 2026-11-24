from __future__ import annotations

from collections.abc import Mapping
from typing import Any, Self, TypeVar

from attrs import define as _attrs_define
from attrs import field as _attrs_field
from typing_extensions import Self

T = TypeVar("T", bound="FailureCountsByCategory")


@_attrs_define
class FailureCountsByCategory:
    """Classified failures per FailureCategory. Every category is present, with 0 when it has none.

    Attributes:
        budget_exceeded (int):
        contract_error (int):
        auth_failure (int):
        storage (int):
        wasm_trap (int):
        other (int):
    """

    budget_exceeded: int
    contract_error: int
    auth_failure: int
    storage: int
    wasm_trap: int
    other: int
    additional_properties: dict[str, Any] = _attrs_field(init=False, factory=dict)

    def to_dict(self) -> dict[str, Any]:
        budget_exceeded = self.budget_exceeded

        contract_error = self.contract_error

        auth_failure = self.auth_failure

        storage = self.storage

        wasm_trap = self.wasm_trap

        other = self.other

        field_dict: dict[str, Any] = {}
        field_dict.update(self.additional_properties)
        field_dict.update(
            {
                "budget_exceeded": budget_exceeded,
                "contract_error": contract_error,
                "auth_failure": auth_failure,
                "storage": storage,
                "wasm_trap": wasm_trap,
                "other": other,
            }
        )

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        d = dict(src_dict)
        budget_exceeded = d.pop("budget_exceeded")

        contract_error = d.pop("contract_error")

        auth_failure = d.pop("auth_failure")

        storage = d.pop("storage")

        wasm_trap = d.pop("wasm_trap")

        other = d.pop("other")

        failure_counts_by_category = cls(
            budget_exceeded=budget_exceeded,
            contract_error=contract_error,
            auth_failure=auth_failure,
            storage=storage,
            wasm_trap=wasm_trap,
            other=other,
        )

        failure_counts_by_category.additional_properties = d
        return failure_counts_by_category

    @property
    def additional_keys(self) -> list[str]:
        return list(self.additional_properties.keys())

    def __getitem__(self, key: str) -> Any:
        return self.additional_properties[key]

    def __setitem__(self, key: str, value: Any) -> None:
        self.additional_properties[key] = value

    def __delitem__(self, key: str) -> None:
        del self.additional_properties[key]

    def __contains__(self, key: str) -> bool:
        return key in self.additional_properties
