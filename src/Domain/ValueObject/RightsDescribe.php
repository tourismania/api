<?php

declare(strict_types=1);
declare(ticks=1000);

namespace App\Domain\ValueObject;

readonly class RightsDescribe
{
    public function __construct(
        public bool $isSuperAdmin,
    ) {
    }
}
