from __future__ import annotations

from collections.abc import Mapping
from typing import TYPE_CHECKING, Any, TypeVar

from attrs import define as _attrs_define
from attrs import field as _attrs_field
from typing_extensions import Self

from ..types import UNSET, Unset

if TYPE_CHECKING:
    from ..models.verification_expected_input import VerificationExpectedInput
    from ..models.verification_source_input import VerificationSourceInput


T = TypeVar("T", bound="ContractVerificationRequest")


@_attrs_define
class ContractVerificationRequest:
    """
    Attributes:
        source (VerificationSourceInput):
        artifact (str | Unset): Built `.wasm` path relative to the crate directory, bypassing
            artifact discovery in multi-artifact workspaces.
        expected (VerificationExpectedInput | Unset):
    """

    source: VerificationSourceInput
    artifact: str | Unset = UNSET
    expected: VerificationExpectedInput | Unset = UNSET
    additional_properties: dict[str, Any] = _attrs_field(init=False, factory=dict)

    def to_dict(self) -> dict[str, Any]:
        source = self.source.to_dict()

        artifact = self.artifact

        expected: dict[str, Any] | Unset = UNSET
        if not isinstance(self.expected, Unset):
            expected = self.expected.to_dict()

        field_dict: dict[str, Any] = {}
        field_dict.update(self.additional_properties)
        field_dict.update(
            {
                "source": source,
            }
        )
        if artifact is not UNSET:
            field_dict["artifact"] = artifact
        if expected is not UNSET:
            field_dict["expected"] = expected

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        from ..models.verification_expected_input import (
            VerificationExpectedInput,
        )
        from ..models.verification_source_input import (
            VerificationSourceInput,
        )

        d = dict(src_dict)
        source = VerificationSourceInput.from_dict(d.pop("source"))

        artifact = d.pop("artifact", UNSET)

        _expected = d.pop("expected", UNSET)
        expected: VerificationExpectedInput | Unset
        if isinstance(_expected, Unset):
            expected = UNSET
        else:
            expected = VerificationExpectedInput.from_dict(_expected)

        contract_verification_request = cls(
            source=source,
            artifact=artifact,
            expected=expected,
        )

        contract_verification_request.additional_properties = d
        return contract_verification_request

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
