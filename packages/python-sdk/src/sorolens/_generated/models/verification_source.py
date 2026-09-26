from __future__ import annotations

from collections.abc import Mapping
from typing import Any, TypeVar

from attrs import define as _attrs_define
from attrs import field as _attrs_field
from typing_extensions import Self

from ..models.verification_source_kind import VerificationSourceKind
from ..types import UNSET, Unset

T = TypeVar("T", bound="VerificationSource")


@_attrs_define
class VerificationSource:
    """
    Attributes:
        kind (VerificationSourceKind):
        ref (str | Unset): Archive filename or resolved Git commit.
        digest (str | Unset): SHA-256 digest of the submitted source.
    """

    kind: VerificationSourceKind
    ref: str | Unset = UNSET
    digest: str | Unset = UNSET
    additional_properties: dict[str, Any] = _attrs_field(init=False, factory=dict)

    def to_dict(self) -> dict[str, Any]:
        kind = self.kind.value

        ref = self.ref

        digest = self.digest

        field_dict: dict[str, Any] = {}
        field_dict.update(self.additional_properties)
        field_dict.update(
            {
                "kind": kind,
            }
        )
        if ref is not UNSET:
            field_dict["ref"] = ref
        if digest is not UNSET:
            field_dict["digest"] = digest

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        d = dict(src_dict)
        kind = VerificationSourceKind(d.pop("kind"))

        ref = d.pop("ref", UNSET)

        digest = d.pop("digest", UNSET)

        verification_source = cls(
            kind=kind,
            ref=ref,
            digest=digest,
        )

        verification_source.additional_properties = d
        return verification_source

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
