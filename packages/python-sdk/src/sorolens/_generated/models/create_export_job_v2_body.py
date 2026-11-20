from __future__ import annotations

from collections.abc import Mapping
from typing import TYPE_CHECKING, Any, Self, TypeVar

from attrs import define as _attrs_define
from attrs import field as _attrs_field
from typing_extensions import Self

from ..models.create_export_job_v2_body_format import CreateExportJobV2BodyFormat
from ..models.create_export_job_v2_body_type import CreateExportJobV2BodyType
from ..types import UNSET, Unset

if TYPE_CHECKING:
    from ..models.create_export_job_v2_body_filters import CreateExportJobV2BodyFilters


T = TypeVar("T", bound="CreateExportJobV2Body")


@_attrs_define
class CreateExportJobV2Body:
    """
    Attributes:
        type_ (CreateExportJobV2BodyType):
        format_ (CreateExportJobV2BodyFormat):
        filters (CreateExportJobV2BodyFilters | Unset):
    """

    type_: CreateExportJobV2BodyType
    format_: CreateExportJobV2BodyFormat
    filters: CreateExportJobV2BodyFilters | Unset = UNSET
    additional_properties: dict[str, Any] = _attrs_field(init=False, factory=dict)

    def to_dict(self) -> dict[str, Any]:
        type_ = self.type_.value

        format_ = self.format_.value

        filters: dict[str, Any] | Unset = UNSET
        if not isinstance(self.filters, Unset):
            filters = self.filters.to_dict()

        field_dict: dict[str, Any] = {}
        field_dict.update(self.additional_properties)
        field_dict.update(
            {
                "type": type_,
                "format": format_,
            }
        )
        if filters is not UNSET:
            field_dict["filters"] = filters

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        from ..models.create_export_job_v2_body_filters import (
            CreateExportJobV2BodyFilters,
        )

        d = dict(src_dict)
        type_ = CreateExportJobV2BodyType(d.pop("type"))

        format_ = CreateExportJobV2BodyFormat(d.pop("format"))

        _filters = d.pop("filters", UNSET)
        filters: CreateExportJobV2BodyFilters | Unset
        if isinstance(_filters, Unset):
            filters = UNSET
        else:
            filters = CreateExportJobV2BodyFilters.from_dict(_filters)

        create_export_job_v2_body = cls(
            type_=type_,
            format_=format_,
            filters=filters,
        )

        create_export_job_v2_body.additional_properties = d
        return create_export_job_v2_body

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
